package consensus

import (
	"runtime"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("pow")

// initPoW inits the main of consensus, shouldn't be shut down.
func (pow *PoWork) initPoW(workerNum int) {
	log.Info("Initializing PoWork pow")

	if workerNum == 0 {
		workerNum = runtime.NumCPU()
	}
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
		pow.minerMod.start(pow.getBlockTemplate())
	}
}

// getBlockTemplate is a generator of new block. But the generated block has no nonce.
func (pow *PoWork) getBlockTemplate() *ngtypes.Block {
	pow.RLock()
	defer pow.RUnlock()

	currentBlock := pow.chain.GetLatestBlock()

	currentBlockHash, err := currentBlock.CalculateHash()
	if err != nil {
		log.Error(err)
	}

	newDiff := ngtypes.GetNextDiff(currentBlock)
	newHeight := currentBlock.Header.Height + 1

	newBareBlock := ngtypes.NewBareBlock(
		newHeight,
		currentBlockHash,
		newDiff,
	)

	extraData := []byte("ngCore")

	Gen := pow.createGenerateTx(pow.PrivateKey, extraData)
	txsWithGen := append([]*ngtypes.Tx{Gen}, pow.txpool.GetPackTxs()...)

	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	return newUnsealingBlock
}

// MinedNewBlock means the consensus mined new block and need to add it into the chain.
func (pow *PoWork) minedNewBlock(block *ngtypes.Block) {
	pow.Lock()
	defer pow.Unlock()

	if err := pow.checkBlock(block); err != nil {
		log.Warn("Malformed block mined:", err)
		return
	}

	prevBlock, err := pow.chain.GetBlockByHash(block.Header.PrevBlockHash)
	if err != nil {
		log.Error("cannot find the prevBlock for new block, rejected:", err)
		return
	}

	if prevBlock == nil {
		log.Warn("Malformed block mined: PrevBlockHash")
		return
	}

	hash, _ := block.CalculateHash()
	log.Infof("Mined a new Block: %x@%d", hash, block.GetHeight())

	err = pow.PutNewBlock(block) // chain will verify the block
	if err != nil {
		log.Warn(err)
		return
	}

	pow.localNode.BroadcastBlock(block)
}
