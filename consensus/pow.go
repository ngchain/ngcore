package consensus

import (
	"runtime"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("consensus")

// InitPoW inits the main of consensus, shouldn't be shut down.
func (c *PoWork) InitPoW(workerNum int) {
	log.Info("Initializing PoWork consensus")

	if workerNum == 0 {
		workerNum = runtime.NumCPU()
	}

	if c.isMining {
		c.miner = miner.NewMiner(workerNum)
		c.miner.Start(c.getBlockTemplate())

		go func() {
			for {
				b := <-c.miner.FoundBlockCh
				c.minedNewBlock(b)

				if c.isMining {
					c.miner.Start(c.getBlockTemplate())
				}
			}
		}()
	}
}

// Stop the pow consensus.
func (c *PoWork) Stop() {
	if c.isMining {
		log.Info("mining stopping")
		c.miner.Stop()
	}
}

// Resume the pow consensus.
func (c *PoWork) Resume() {
	if c.isMining {
		log.Info("mining resuming")
		c.miner.Start(c.getBlockTemplate())
	}
}

// getBlockTemplate is a generator of new block. But the generated block has no nonce.
func (c *PoWork) getBlockTemplate() *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	currentBlock := c.chain.GetLatestBlock()

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

	Gen := c.createGenerateTx(c.PrivateKey, extraData)
	txsWithGen := append([]*ngtypes.Tx{Gen}, c.txpool.GetPackTxs()...)

	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	return newUnsealingBlock
}

// MinedNewBlock means the consensus mined new block and need to add it into the chain.
func (c *PoWork) minedNewBlock(block *ngtypes.Block) {
	c.Lock()
	defer c.Unlock()

	if err := c.checkBlock(block); err != nil {
		log.Warn("Malformed block mined:", err)
		return
	}

	prevBlock, err := c.chain.GetBlockByHash(block.Header.PrevBlockHash)
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

	err = c.chain.MinedNewBlock(block)
	if err != nil {
		log.Warn(err)
		return
	}

	err = c.sheetManager.HandleTxs(block.Txs...)
	if err != nil {
		log.Warn(err)
		return
	}
}
