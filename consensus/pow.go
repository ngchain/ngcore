package consensus

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/secp256k1"
)

var log = logging.Logger("pow")

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	PoWorkConfig

	syncMod  *syncModule
	minerMod *miner.Miner

	Chain     *ngchain.Chain
	Pool      *ngpool.TxPool
	State     *ngstate.State
	LocalNode *ngp2p.LocalNode

	db *badger.DB

	// for miner
	foundBlockCh chan *ngtypes.Block
}

type PoWorkConfig struct {
	Network                     ngtypes.NetworkType
	DisableConnectingBootstraps bool
	MiningThread                int
	PrivateKey                  *secp256k1.PrivateKey
}

// InitPoWConsensus creates and initializes the PoW consensus.
func InitPoWConsensus(db *badger.DB, chain *ngchain.Chain, pool *ngpool.TxPool, state *ngstate.State, localNode *ngp2p.LocalNode, config PoWorkConfig) *PoWork {
	pow := &PoWork{
		RWMutex:      sync.RWMutex{},
		PoWorkConfig: config,
		syncMod:      nil,
		minerMod:     nil,
		Chain:        chain,
		Pool:         pool,
		State:        state,
		LocalNode:    localNode,

		db: db,

		foundBlockCh: make(chan *ngtypes.Block),
	}

	// init sync before miner to prevent bootstrap sync from mining job update
	pow.syncMod = newSyncModule(pow, localNode)
	if !pow.DisableConnectingBootstraps {
		pow.syncMod.bootstrap()
	}

	pow.minerMod = miner.NewMiner(pow.MiningThread, pow.foundBlockCh)

	return pow
}

// MiningOff stops the pow consensus.
func (pow *PoWork) MiningOff() {
	if pow.minerMod != nil {
		pow.minerMod.Stop()
	}
}

// MiningOn resumes the pow consensus.
func (pow *PoWork) MiningOn() {
	if pow.minerMod != nil {
		newBlock := pow.GetBlockTemplate()
		go pow.minerMod.Start(newBlock)
	}
}

// MiningUpdate updates the mining work
func (pow *PoWork) MiningUpdate() {
	pow.MiningOff()
	pow.MiningOn()
}

// GetBlockTemplate is a generator of new block. But the generated block has no nonce.
func (pow *PoWork) GetBlockTemplate() *ngtypes.Block {
	pow.RLock()
	defer pow.RUnlock()

	currentBlock := pow.Chain.GetLatestBlock()

	currentBlockHash := currentBlock.Hash()

	newDiff := ngtypes.GetNextDiff(currentBlock)
	newHeight := currentBlock.Height + 1

	newBareBlock := ngtypes.NewBareBlock(
		pow.Network,
		newHeight,
		currentBlockHash,
		newDiff,
	)

	var extraData []byte // FIXME

	Gen := pow.createGenerateTx(extraData)
	txs := pow.Pool.GetPack().Txs
	txsWithGen := append([]*ngtypes.Tx{Gen}, txs...)

	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	return newUnsealingBlock
}

// GoLoop ignites all loops
func (pow *PoWork) GoLoop() {
	go pow.eventLoop()
	go pow.syncMod.loop()
}

// channel receiver for broadcasts events
func (pow *PoWork) eventLoop() {
	for {
		select {
		case block := <-pow.LocalNode.OnBlock:
			err := pow.Chain.ApplyBlock(block)
			if err != nil {
				log.Warnf("failed to put new block from p2p network: %s", err)
				continue
			}

			// update miner work
			go pow.MiningUpdate()

		case tx := <-pow.LocalNode.OnTx:
			err := pow.Pool.PutTx(tx)
			if err != nil {
				log.Warnf("failed to put new tx from p2p network: %s", err)
			}

		case newBlock := <-pow.foundBlockCh:
			err := pow.MinedNewBlock(newBlock)
			if err != nil {
				log.Warnf("error on handling the mined block: %s", err)
			}

			// assign new job
			blockTemplate := pow.GetBlockTemplate()
			pow.minerMod.Start(blockTemplate)
		}
	}
}

// MinedNewBlock means the consensus mined new block and need to add it into the chain.
func (pow *PoWork) MinedNewBlock(block *ngtypes.Block) error {
	// check block first
	err := pow.db.Update(func(txn *badger.Txn) error {
		// check block first
		if err := pow.Chain.CheckBlock(block); err != nil {
			return err
		}

		// block is valid
		err := ngblocks.PutNewBlock(txn, block)
		if err != nil {
			return err
		}

		err = pow.State.Upgrade(txn, block) // handle Block Txs inside
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	hash := block.Hash()
	fmt.Printf("Mined a new Block: %x@%d \n", hash, block.GetHeight())
	log.Warnf("Mined a new Block: %x@%d", hash, block.GetHeight())

	err = pow.LocalNode.BroadcastBlock(block)
	if err != nil {
		return fmt.Errorf("failed to broadcast the new mined block")
	}

	return nil
}
