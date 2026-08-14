package consensus

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"github.com/ngchain/secp256k1"
	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/blockchain"
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

	ctx    context.Context
	cancel context.CancelFunc
}

type PoWorkConfig struct {
	Network                     ngtypes.Network
	StrictMode                  bool
	SnapshotMode                bool
	DisableConnectingBootstraps bool
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

		ctx:    ctx,
		cancel: cancel,
	}

	// init sync before miner to prevent bootstrap sync from mining job update
	pow.SyncMod = newSyncModule(pow, localNode)
	if !pow.DisableConnectingBootstraps {
		pow.SyncMod.bootstrap()
	}

	// run reporter
	go pow.reportLoop()

	return pow
}

// GetBlockTemplate is a generator of new block. But the generated block has no nonce.
func (pow *PoWork) GetBlockTemplate(privateKey *secp256k1.PrivateKey) ngtypes.Block {
	currentBlock := pow.Chain.GetLatestBlock()

	currentBlockHash := currentBlock.GetHash()

	blockTime := uint64(time.Now().Unix())

	blockHeight := currentBlock.GetHeight() + 1
	newDiff := ngtypes.GetNextDiff(blockHeight, blockTime, currentBlock.(*ngtypes.FullBlock))

	newBlock := ngtypes.NewBareBlock(
		pow.Network,
		blockHeight,
		blockTime,
		currentBlockHash,
		newDiff,
	)

	var extraData []byte // FIXME

	genTx := CreateGenerateTx(pow.Network, privateKey, blockHeight, extraData)
	txs := pow.Pool.GetPack(blockHeight)
	txsWithGen := append([]*ngtypes.FullTx{genTx}, txs...)

	err := newBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	return newBlock
}

// GetBlockTemplate is a generator of new block. But the generated block has no nonce.
func (pow *PoWork) GetBareBlockTemplateWithTxs() (bareBlock *ngtypes.FullBlock, txs []*ngtypes.FullTx) {
	currentBlock := pow.Chain.GetLatestBlock()

	currentBlockHash := currentBlock.GetHash()

	blockTime := uint64(time.Now().Unix())

	blockHeight := currentBlock.GetHeight() + 1
	newDiff := ngtypes.GetNextDiff(blockHeight, blockTime, currentBlock.(*ngtypes.FullBlock))

	bareBlock = ngtypes.NewBareBlock(
		pow.Network,
		blockHeight,
		blockTime,
		currentBlockHash,
		newDiff,
	)

	// var extraData []byte // FIXME

	txs = pow.Pool.GetPack(blockHeight)
	// genTx := pow.createGenerateTx(privateKey, blockHeight, extraData)

	// txsWithGen := append([]*ngtypes.FullTx{genTx}, txs...)

	// err := newBlock.ToUnsealing(txsWithGen)
	// if err != nil {
	// 	log.Error(err)
	// }

	return
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
				if err := pow.ImportBlock(block); err != nil {
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
}

var ErrChainOnSyncing = errors.New("chain is syncing")

// MinedNewBlock means the local (from rpc) mined new block and need to add it into the chain.
// called by submitBlock and submitWork
func (pow *PoWork) MinedNewBlock(block *ngtypes.FullBlock) error {
	if pow.SyncMod.Locker.IsLocked() {
		return fmt.Errorf("cannot import mined block: %w", ErrChainOnSyncing)
	}

	// ApplyBlock checks and imports the block (with fork choice) atomically
	err := pow.Chain.ApplyBlock(block)
	if err != nil {
		return err
	}

	hash := block.GetHash()
	log.Warnf("mined a new block: %x@%d", hash, block.GetHeight())

	pow.Pool.Reset()

	err = pow.LocalNode.BroadcastBlock(block)
	if err != nil {
		return fmt.Errorf("%w: failed to broadcast the new mined block", err)
	}

	return nil
}

func (pow *PoWork) ImportBlock(b ngtypes.Block) error {
	block := b.(*ngtypes.FullBlock)

	if pow.SyncMod.Locker.IsLocked() {
		return errors.Wrap(ErrChainOnSyncing, "cannot import external block")
	}

	err := pow.Chain.ApplyBlock(block)
	if err != nil {
		return err
	}

	return nil
}
