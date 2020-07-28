package consensus

import (
	"fmt"
	"sync"

	"github.com/ngchain/secp256k1"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var log = logging.Logger("pow")

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	syncMod  *syncModule
	minerMod *minerModule

	PrivateKey *secp256k1.PrivateKey
}

var pow *PoWork

// NewPoWConsensus creates and initializes the PoW consensus.
func NewPoWConsensus(miningThread int, privateKey *secp256k1.PrivateKey, isBootstrapNode bool) *PoWork {
	pow = &PoWork{
		RWMutex:    sync.RWMutex{},
		PrivateKey: privateKey,

		syncMod:  nil,
		minerMod: nil,
	}

	// init sync before miner to prevent bootstrap sync from mining job update
	pow.syncMod = newSyncModule(pow, isBootstrapNode)
	pow.minerMod = newMinerModule(pow, miningThread)

	return pow
}

// GetPoWConsensus creates a new proof of work consensus manager.
func GetPoWConsensus() *PoWork {
	if pow == nil {
		panic("pow has not initialized")
	}

	return pow
}

// MiningOff stops the pow consensus.
func (pow *PoWork) MiningOff() {
	if pow.minerMod != nil {
		pow.minerMod.stop()
	}
}

// MiningOn resumes the pow consensus.
func (pow *PoWork) MiningOn() {
	if pow.minerMod != nil {
		go pow.minerMod.start(pow.GetBlockTemplate())
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

	currentBlock := storage.GetChain().GetLatestBlock()

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
	txs := ngstate.GetActiveState().GetPool().GetPack().Txs
	txsWithGen := append([]*ngtypes.Tx{Gen}, txs...)

	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	return newUnsealingBlock
}

// MinedNewBlock means the consensus mined new block and need to add it into the chain.
func (pow *PoWork) MinedNewBlock(block *ngtypes.Block) error {
	pow.Lock()
	defer pow.Unlock()

	// check block first
	if err := pow.checkBlock(block); err != nil {
		return fmt.Errorf("malformed block mined: %s", err)
	}

	prevBlock, err := storage.GetChain().GetBlockByHash(block.PrevBlockHash)
	if err != nil {
		log.Error("cannot find the prevBlock for new block, rejected:", err)
		return err
	}

	if prevBlock == nil {
		return fmt.Errorf("malformed block mined: cannot find PrevBlock %x", block.PrevBlockHash)
	}

	// block is valid
	hash := block.Hash()
	fmt.Printf("Mined a new Block: %x@%d \n", hash, block.GetHeight())
	log.Warnf("Mined a new Block: %x@%d", hash, block.GetHeight())

	err = storage.GetChain().PutNewBlock(block) // chain will verify the block
	if err != nil {
		return fmt.Errorf("failed put new block into chain: %s", err)
	}

	err = ngp2p.GetLocalNode().BroadcastBlock(block)
	if err != nil {
		return fmt.Errorf("failed to broadcast the new mined block")
	}

	return nil
}

// GoLoop ignites all loops
func (pow *PoWork) GoLoop() {
	go pow.eventLoop()
	go pow.syncMod.loop()
}

// channel receiver for broadcasts events
func (pow *PoWork) eventLoop() {
	for {
		if ngp2p.GetLocalNode().OnBlock == nil || ngp2p.GetLocalNode().OnTx == nil {
			panic("event chan is nil")
		}

		select {
		case block := <-ngp2p.GetLocalNode().OnBlock:
			err := pow.ApplyBlock(block)
			if err != nil {
				log.Warnf("failed to put new block from p2p network: %s", err)
			}
		case tx := <-ngp2p.GetLocalNode().OnTx:
			err := ngstate.GetActiveState().GetPool().PutTx(tx)
			if err != nil {
				log.Warnf("failed to put new tx from p2p network: %s", err)
			}
		}
	}
}
