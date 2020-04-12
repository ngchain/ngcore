package consensus

import (
	"runtime"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.MustGetLogger("consensus")

// InitPoW inits the main of consensus, shouldn't be shut down
func (c *Consensus) InitPoW(workerNum int) {
	log.Info("Initializing PoW consensus")

	if workerNum == 0 {
		workerNum = runtime.NumCPU()
	}

	if c.isMining {
		c.miner = miner.NewMiner(workerNum)
		c.miner.Start(c.getBlockTemplate())

		go func() {
			for {
				b := <-c.miner.FoundBlockCh
				c.MinedNewBlock(b)

				if c.isMining {
					c.miner.Start(c.getBlockTemplate())
				}
			}
		}()
	}
}

// Stop the consensus
func (c *Consensus) Stop() {
	if c.isMining {
		log.Info("mining stopping")
		c.miner.Stop()
	}
}

// Resume the consensus
func (c *Consensus) Resume() {
	if c.isMining {
		log.Info("mining resuming")
		c.miner.Start(c.getBlockTemplate())
	}
}

// getBlockTemplate is a generator of new block. But the generated block has no nonce
func (c *Consensus) getBlockTemplate() *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	currentBlock := c.Chain.GetLatestBlock()
	currentBlockHash, _ := currentBlock.CalculateHash()

	newBlockHeight := currentBlock.Header.Height + 1

	currentVault := c.Chain.GetLatestVault()
	currentVaultHash, _ := currentVault.CalculateHash()

	currentBlocksVault, _ := c.Chain.GetVaultByHash(currentBlock.Header.PrevVaultHash)
	newTarget := ngtypes.GetNextTarget(currentBlock, currentBlocksVault)

	newBareBlock := ngtypes.NewBareBlock(
		newBlockHeight,
		currentBlockHash,
		currentVaultHash,
		newTarget,
	)

	extraData := []byte("ngCore")

	Gen := c.createGenerateTx(c.PrivateKey, extraData)
	txsWithGen := append([]*ngtypes.Tx{Gen}, c.TxPool.GetPackTxs()...)
	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	if newUnsealingBlock.IsHead() {
		log.Infof("using new vault: %x", newBareBlock.Header.PrevVaultHash)
	}

	return newUnsealingBlock
}

// MinedNewBlock means the consensus mined new block and need to add it into the chain
func (c *Consensus) MinedNewBlock(block *ngtypes.Block) {
	c.Lock()
	defer c.Unlock()

	if err := c.checkBlock(block); err != nil {
		log.Warning("Malformed block mined:", err)
		return
	}

	prevBlock, err := c.Chain.GetBlockByHash(block.Header.PrevBlockHash)
	if err != nil {
		log.Error("cannot find the prevBlock for new block, rejected:", err)
		return
	}

	if prevBlock == nil {
		log.Warning("Malformed block mined: PrevBlockHash")
		return
	}

	prevVault, err := c.Chain.GetVaultByHash(block.Header.PrevVaultHash)
	if err != nil {
		log.Error("cannot find the prevVault for new block, rejected:", err)
		return
	}
	if prevVault == nil {
		log.Warning("Malformed block mined: PrevVaultHash")
		return
	}

	hash, _ := block.CalculateHash()
	log.Infof("Mined a new Block: %x@%d", hash, block.GetHeight())

	// TODO: vault should be generated when made the template
	if block.Header.IsHead() {
		err := c.HandleVault(prevVault)
		if err != nil {
			log.Error(err)
		}
	}

	err = c.Chain.MinedNewBlock(block)
	if err != nil {
		log.Warning(err)
		return
	}

	err = c.HandleTxs(block.Txs...)
	if err != nil {
		log.Warning(err)
		return
	}

	if block.Header.IsTail() {
		currentVault := c.GenNewVaultCandidate(block.Header.Height/ngtypes.BlockCheckRound, block.Header.PrevVaultHash)
		err := c.checkVault(currentVault)
		if err != nil {
			panic(err)
		}
		err = c.Chain.MinedNewVault(currentVault)
		if err != nil {
			panic(err)
		}
	}
}

// GenNewVaultCandidate is called when the reached a checkpoint, then generate a
func (c *Consensus) GenNewVaultCandidate(prevVaultHeight uint64, prevVaultHash []byte) *ngtypes.Vault {
	sheet, err := c.GenerateNewSheet()
	if err != nil {
		log.Error(err)
		return nil
	}
	newVault := ngtypes.NewVault(prevVaultHeight, prevVaultHash, sheet)
	hash, _ := newVault.CalculateHash()
	log.Infof("Generated a new Vault@%d, %x", newVault.GetHeight(), hash)
	return newVault
}
