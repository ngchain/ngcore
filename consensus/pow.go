package consensus

import (
	"context"
	"fmt"
	"math/big"
	"time"

	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("pow")

// PoWork is a proof on work consensus manager.
type PoWork struct {
	PoWorkConfig

	SyncMod *syncModule

	Chain     *blockchain.Chain
	Pool      *ngpool.TxPool
	State     *ngstate.State
	LocalNode *ngp2p.LocalNode

	db *bbolt.DB

	orphans *orphanPool

	ctx    context.Context
	cancel context.CancelFunc
}

type PoWorkConfig struct {
	Network                     ngtypes.Network
	StrictMode                  bool
	SnapshotMode                bool
	DisableConnectingBootstraps bool

	// MinerExtraData is embedded in the generate tx of locally mined
	// blocks (capped by TxMaxExtraSize)
	MinerExtraData []byte
}

// InitPoWConsensus creates and initializes the PoW consensus.
func InitPoWConsensus(db *bbolt.DB, chain *blockchain.Chain, pool *ngpool.TxPool, state *ngstate.State, localNode *ngp2p.LocalNode, config PoWorkConfig) *PoWork {
	ctx, cancel := context.WithCancel(context.Background())

	pow := &PoWork{
		PoWorkConfig: config,
		SyncMod:      nil,
		Chain:        chain,
		Pool:         pool,
		State:        state,
		LocalNode:    localNode,

		db: db,

		orphans: newOrphanPool(),

		ctx:    ctx,
		cancel: cancel,
	}

	// any tip movement (import or reorg) deprecates the height-locked txs and
	// re-relays any held reveals at the new height (fire-and-forget commit-reveal)
	chain.OnTipChanged = pool.OnTipChanged

	// init sync before miner to prevent bootstrap sync from mining job update
	pow.SyncMod = newSyncModule(pow, localNode)
	if !pow.DisableConnectingBootstraps {
		pow.SyncMod.bootstrap()
	}

	// run reporter
	go pow.reportLoop()

	return pow
}

// templateBlockTime picks the mining timestamp in unix-MILLISECONDS, but
// always strictly after the parent so back-to-back templates within the
// same millisecond stay monotonic (the retarget reads this interval)
func templateBlockTime(parent *ngtypes.FullBlock) uint64 {
	blockTime := uint64(time.Now().UnixMilli())
	if blockTime <= parent.BlockHeader.Timestamp {
		blockTime = parent.BlockHeader.Timestamp + 1
	}

	return blockTime
}

// nextBaseFeeBytes computes the child block's burn-only base fee (ForkFeeMarket)
// from its parent, as minimal big-endian bytes ready to drop into the header.
// Pre-fork it is MinBaseFee. It is part of the pow preimage, so a template must
// set it before sealing.
func (pow *PoWork) nextBaseFeeBytes(parent *ngtypes.FullBlock) []byte {
	return ngtypes.NextBaseFee(
		pow.Network,
		parent.GetHeight(),
		new(big.Int).SetBytes(parent.BlockHeader.BaseFee),
		ngtypes.BlockUsedBytes(parent),
	).Bytes()
}

// GetBlockTemplate is a generator of new block. But the generated block has no nonce.
func (pow *PoWork) GetBlockTemplate(privateKey *ngtypes.PrivateKey) ngtypes.Block {
	currentBlock := pow.Chain.GetLatestBlock()

	currentBlockHash := currentBlock.GetHash()

	blockTime := templateBlockTime(currentBlock.(*ngtypes.FullBlock))

	blockHeight := currentBlock.GetHeight() + 1
	newDiff := ngtypes.GetNextDiff(blockHeight, blockTime, currentBlock.(*ngtypes.FullBlock))

	newBlock := ngtypes.NewBareBlock(
		pow.Network,
		blockHeight,
		blockTime,
		currentBlockHash,
		newDiff,
	)
	// seal the child's base fee into the header before pow (it is in the preimage)
	newBlock.BlockHeader.BaseFee = pow.nextBaseFeeBytes(currentBlock.(*ngtypes.FullBlock))

	// GHOST: reference recent orphaned blocks so their work counts toward
	// this chain (folds uncle difficulty into fork choice) and pays their
	// miners. Must run before sealing — UnclesHash is part of the pow preimage.
	var uncleRewards []*ngtypes.FullTx
	if uncles, err := pow.Chain.CollectUncles(); err != nil {
		log.Warnf("failed to collect uncles: %s", err)
	} else {
		newBlock.SetUncles(uncles)
		uncleRewards = buildUncleRewardTxs(pow.Network, uncles, blockHeight)
	}

	// the miner address goes in the header so future nephews can pay it
	newBlock.SetCoinbase(ngtypes.NewAddress(privateKey))

	genTx := CreateGenerateTx(pow.Network, privateKey, blockHeight, pow.MinerExtraData)
	txs := pow.Pool.GetPack(blockHeight)
	txsWithGen := make([]*ngtypes.FullTx, 0, 1+len(uncleRewards)+len(txs))
	txsWithGen = append(txsWithGen, genTx)
	txsWithGen = append(txsWithGen, uncleRewards...)
	txsWithGen = append(txsWithGen, txs...)

	// the blind commitments ride the block's Commits list; ToUnsealing folds
	// them into the content root alongside the txs
	newBlock.SetCommits(pow.Pool.GetCommitPack(blockHeight))

	err := newBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	// the header commits to the POST-state root, which depends on this block's
	// own contents (incl. the miner's generate), so it must be sealed into the
	// preimage BEFORE pow. DryApplyRoot applies the fully-assembled candidate
	// in a throwaway txn and rolls back, returning the root it would produce.
	root, err := ngstate.DryApplyRoot(pow.State, newBlock)
	if err != nil {
		log.Errorf("failed to compute the template state root: %s", err)
	} else {
		newBlock.BlockHeader.StateRoot = root
	}

	return newBlock
}

// GetBlockTemplate is a generator of new block. But the generated block has no nonce.
func (pow *PoWork) GetBareBlockTemplateWithTxs() (bareBlock *ngtypes.FullBlock, txs []*ngtypes.FullTx) {
	currentBlock := pow.Chain.GetLatestBlock()

	currentBlockHash := currentBlock.GetHash()

	blockTime := templateBlockTime(currentBlock.(*ngtypes.FullBlock))

	blockHeight := currentBlock.GetHeight() + 1
	newDiff := ngtypes.GetNextDiff(blockHeight, blockTime, currentBlock.(*ngtypes.FullBlock))

	bareBlock = ngtypes.NewBareBlock(
		pow.Network,
		blockHeight,
		blockTime,
		currentBlockHash,
		newDiff,
	)
	// seal the child's base fee into the header before pow (it is in the preimage)
	bareBlock.BlockHeader.BaseFee = pow.nextBaseFeeBytes(currentBlock.(*ngtypes.FullBlock))

	// GHOST: attach recent orphans as uncles before the miner seals, and
	// prepend the (unsigned) uncle-reward generates to the tx pack so the
	// miner includes them ahead of the pool txs. The miner adds its own
	// signed generate and sets the header Coinbase before sealing.
	// the blind commitments ride the template so the miner seals over the
	// content root that folds them in
	bareBlock.SetCommits(pow.Pool.GetCommitPack(blockHeight))

	if uncles, err := pow.Chain.CollectUncles(); err != nil {
		log.Warnf("failed to collect uncles: %s", err)
	} else {
		bareBlock.SetUncles(uncles)
		uncleRewards := buildUncleRewardTxs(pow.Network, uncles, blockHeight)
		txs = append(uncleRewards, pow.Pool.GetPack(blockHeight)...)
		return
	}

	txs = pow.Pool.GetPack(blockHeight)
	return
}

// AssembleWork folds the miner's own (signed) generate tx into a fresh
// template and returns a FULLY-ASSEMBLED unsealing block with its post-state
// StateRoot sealed into the header — everything but the nonce. The StateRoot
// is part of the pow preimage and depends on this block's contents (incl. this
// very generate), so getWork must set it here, before the external miner
// grinds; ng_submitWork then only carries the found nonce. The block's
// Coinbase is taken from the generate's recipient, matching the internal path.
func (pow *PoWork) AssembleWork(gen *ngtypes.FullTx) (*ngtypes.FullBlock, error) {
	bareBlock, txs := pow.GetBareBlockTemplateWithTxs()

	// the miner sealed over its own coinbase (part of the pow preimage); take
	// it from the submitted generate's recipient, matching submitWork's old
	// reconstruction
	bareBlock.SetCoinbase(gen.To)

	if err := bareBlock.ToUnsealing(append([]*ngtypes.FullTx{gen}, txs...)); err != nil {
		return nil, err
	}

	root, err := ngstate.DryApplyRoot(pow.State, bareBlock)
	if err != nil {
		return nil, err
	}
	bareBlock.BlockHeader.StateRoot = root

	return bareBlock, nil
}

// GetChain returns the chain of the PoW consensus.
func (pow *PoWork) GetChain() ngtypes.Chain {
	return pow.Chain
}

// GoLoop ignites all loops.
func (pow *PoWork) GoLoop() {
	go pow.eventLoop()
	go pow.SyncMod.loop(pow.ctx)
}

// Stop shuts the consensus down: all loops exit and the p2p node closes.
// The db handle stays open — it belongs to the caller
func (pow *PoWork) Stop() {
	pow.cancel()

	if err := pow.LocalNode.Close(); err != nil {
		log.Errorf("failed to close the p2p node: %v", err)
	}

	log.Warn("consensus stopped")
}

// channel receiver for broadcasts events.
func (pow *PoWork) eventLoop() {
	go func() {
		for {
			select {
			case block := <-pow.LocalNode.OnBlock:
				if err := pow.safeImportBlock(block); err != nil {
					log.Warnf("failed to put new block from p2p: %s", err)
				}
			case <-pow.ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case tx := <-pow.LocalNode.OnTx:
				if err := pow.Pool.PutTx(tx); err != nil {
					log.Warnf("failed to put new tx from p2p network: %s", err)
				}
			case <-pow.ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case commit := <-pow.LocalNode.OnCommit:
				if err := pow.Pool.PutNewCommitmentFromRemote(commit); err != nil {
					log.Warnf("failed to put new commitment from p2p network: %s", err)
				}
			case <-pow.ctx.Done():
				return
			}
		}
	}()
}

var ErrChainOnSyncing = errors.New("chain is syncing")

// MinedNewBlock means the local (from rpc) mined new block and need to add it into the chain.
// called by submitBlock and submitWork
func (pow *PoWork) MinedNewBlock(block *ngtypes.FullBlock) error {
	if pow.SyncMod.Locker.IsActive() {
		return fmt.Errorf("cannot import mined block: %w", ErrChainOnSyncing)
	}

	// ApplyBlock checks and imports the block (with fork choice) atomically
	err := pow.Chain.ApplyBlock(block)
	if err != nil {
		return err
	}

	hash := block.GetHash()
	log.Warnf("mined a new block: %x@%d", hash, block.GetHeight())

	// the pool is deprecated and reveals re-relayed by chain.OnTipChanged,
	// which ApplyBlock already fired if this block moved the tip — so no
	// explicit Reset here (which would also wrongly drop the pool after
	// mining a mere side block)

	err = pow.LocalNode.BroadcastBlock(block)
	if err != nil {
		return fmt.Errorf("%w: failed to broadcast the new mined block", err)
	}

	return nil
}

// safeImportBlock imports a gossiped (untrusted) block, converting any panic
// during decode/validation into an error so a single malformed block from a
// peer cannot crash the node (GetHash/GetUnsignedHash panic on rlp-encode
// failure; this is the goroutine boundary that must contain that).
func (pow *PoWork) safeImportBlock(b ngtypes.Block) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered from panic while importing a p2p block: %v", r)
			err = fmt.Errorf("panic importing p2p block: %v", r)
		}
	}()
	return pow.ImportBlock(b)
}

func (pow *PoWork) ImportBlock(b ngtypes.Block) error {
	block := b.(*ngtypes.FullBlock)

	// while a sync is running the chain is locked, so a gossiped block cannot
	// be applied now — but dropping it isolates this node from fresh blocks.
	// Park it instead; drainOrphansFromTip (run after the sync unlocks) retries
	// it once its parent has landed.
	if pow.SyncMod.Locker.IsActive() {
		pow.orphans.add(block)
		return nil
	}

	err := pow.Chain.ApplyBlock(block)
	if errors.Is(err, ngblocks.ErrPrevBlockNotExist) {
		// gossip reordering: park the block until its parent arrives
		if pow.orphans.add(block) {
			log.Warnf("parked orphan block@%d %x (waiting for %x)",
				block.GetHeight(), block.GetHash(), block.GetPrevHash())
			return nil
		}
		return err
	}
	if err != nil {
		return err
	}

	pow.importOrphanChildren(block.GetHash())

	return nil
}

// importOrphanChildren cascades the parked blocks whose parent just
// landed, unwinding whole out-of-order bursts
func (pow *PoWork) importOrphanChildren(parentHash []byte) {
	queue := pow.orphans.take(parentHash)
	for len(queue) > 0 {
		child := queue[0]
		queue = queue[1:]

		if err := pow.Chain.ApplyBlock(child); err != nil {
			log.Warnf("parked block@%d %x failed to apply: %s", child.GetHeight(), child.GetHash(), err)
			continue
		}

		queue = append(queue, pow.orphans.take(child.GetHash())...)
	}
}
