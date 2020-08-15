package consensus

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"

	"github.com/ngchain/secp256k1"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("pow")

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	syncMod  *syncModule
	minerMod *minerModule

	storage *badger.DB

	PrivateKey *secp256k1.PrivateKey
}

var pow *PoWork

// InitPoWConsensus creates and initializes the PoW consensus.
func InitPoWConsensus(miningThread int, privateKey *secp256k1.PrivateKey, isBootstrapNode bool) {
	pow = &PoWork{
		RWMutex:    sync.RWMutex{},
		PrivateKey: privateKey,

		syncMod:  nil,
		minerMod: nil,
	}

	// init sync before miner to prevent bootstrap sync from mining job update
	pow.syncMod = newSyncModule(pow, isBootstrapNode)
	pow.minerMod = newMinerModule(pow, miningThread)
}

// MiningOff stops the pow consensus.
func MiningOff() {
	if pow.minerMod != nil {
		pow.minerMod.stop()
	}
}

// MiningOn resumes the pow consensus.
func MiningOn() {
	if pow.minerMod != nil {
		go pow.minerMod.start(GetBlockTemplate())
	}
}

// MiningUpdate updates the mining work
func MiningUpdate() {
	MiningOff()
	MiningOn()
}

// GetBlockTemplate is a generator of new block. But the generated block has no nonce.
func GetBlockTemplate() *ngtypes.Block {
	pow.RLock()
	defer pow.RUnlock()

	currentBlock := ngchain.GetLatestBlock()

	currentBlockHash := currentBlock.Hash()

	newDiff := ngtypes.GetNextDiff(currentBlock)
	newHeight := currentBlock.Height + 1

	newBareBlock := ngtypes.NewBareBlock(
		newHeight,
		currentBlockHash,
		newDiff,
	)

	var extraData []byte // FIXME

	Gen := pow.createGenerateTx(extraData)
	txs := ngpool.GetPack().Txs
	txsWithGen := append([]*ngtypes.Tx{Gen}, txs...)

	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	return newUnsealingBlock
}

// GoLoop ignites all loops
func GoLoop() {
	go eventLoop()
	go pow.syncMod.loop()
}

// channel receiver for broadcasts events
func eventLoop() {
	for {
		if ngp2p.GetLocalNode().OnBlock == nil || ngp2p.GetLocalNode().OnTx == nil {
			panic("event chan is nil")
		}

		select {
		case block := <-ngp2p.GetLocalNode().OnBlock:
			err := ngchain.ApplyBlock(block)
			if err != nil {
				log.Warnf("failed to put new block from p2p network: %s", err)
				continue
			}

			// update miner work
			go MiningUpdate()

		case tx := <-ngp2p.GetLocalNode().OnTx:
			err := ngpool.PutTx(tx)
			if err != nil {
				log.Warnf("failed to put new tx from p2p network: %s", err)
			}
		}
	}
}

// MinedNewBlock means the consensus mined new block and need to add it into the chain.
func MinedNewBlock(block *ngtypes.Block) error {
	// check block first
	err := pow.storage.Update(func(txn *badger.Txn) error {
		// check block first
		if err := ngchain.CheckBlock(block); err != nil {
			return err
		}

		// block is valid
		err := ngblocks.PutNewBlock(txn, block)
		if err != nil {
			return err
		}

		err = ngstate.Upgrade(txn, block) // handle Block Txs inside
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

	err = ngp2p.GetLocalNode().BroadcastBlock(block)
	if err != nil {
		return fmt.Errorf("failed to broadcast the new mined block")
	}

	return nil
}
