package consensus

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var log = logging.Logger("pow")

// MiningOff stops the pow consensus.
func (pow *PoWork) MiningOff() {
	if pow.minerMod != nil {
		pow.minerMod.stop()
	}
}

// MiningOn resumes the pow consensus.
func (pow *PoWork) MiningOn() {
	if pow.minerMod != nil {
		pow.minerMod.start(pow.GetBlockTemplate())
	}
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

	extraData := []byte("ngCore") // FIXME

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

	hash := block.Hash()
	log.Infof("Mined a new Block: %x@%d", hash, block.GetHeight())

	err = storage.GetChain().PutNewBlock(block) // chain will verify the block
	if err != nil {
		return fmt.Errorf("failed put new block into chain %s", err)
	}

	err = ngp2p.GetLocalNode().BroadcastBlock(block)
	if err != nil {
		return fmt.Errorf("failed to broadcast the new mined block")
	}

	return nil
}
