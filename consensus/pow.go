// act as the main of the pow consensus
// pow -
package consensus

import (
	"runtime"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.MustGetLogger("consensus")

// the main of consensus, shouldn't be shut down
func (c *Consensus) InitPoW(workerNum int) {
	log.Info("Initializing PoW consensus")

	if workerNum == 0 {
		workerNum = runtime.NumCPU()
	}

	if c.isMining {
		c.miner = miner.NewMiner(workerNum)
		c.miner.Start(c.GetBlockTemplate())

		go func() {
			for {
				b := <-c.miner.FoundBlockCh
				c.MinedNewBlock(b)

				if c.isMining {
					c.miner.Start(c.GetBlockTemplate())
				}
			}
		}()
	}
}

func (c *Consensus) Stop() {
	if c.isMining {
		log.Info("mining stopping")
		c.miner.Stop()
	}
}

func (c *Consensus) Resume() {
	if c.isMining {
		log.Info("mining resuming")
		c.miner.Start(c.GetBlockTemplate())
	}
}

func (c *Consensus) GetBlockTemplate() *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	currentBlock := c.Chain.GetLatestBlock()
	currentBlockHash, _ := currentBlock.CalculateHash()

	newBlockHeight := currentBlock.Header.Height + 1

	currentVault := c.Chain.GetLatestVault()
	currentVaultHash, _ := currentVault.CalculateHash()

	newTarget := GetNextTarget(currentBlock, currentVault)

	newBareBlock := ngtypes.NewBareBlock(
		newBlockHeight,
		currentBlockHash,
		currentVaultHash,
		newTarget,
	)

	extraData := []byte("ngCore")

	Gen := c.CreateGeneration(c.PrivateKey, newBlockHeight, extraData)
	txsWithGen := append([]*ngtypes.Transaction{Gen}, c.TxPool.GetPackTxs()...)
	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	if newUnsealingBlock.IsHead() {
		log.Infof("using new vault: %x", newBareBlock.Header.PrevVaultHash)
	}

	return newUnsealingBlock
}

// mined new block and need to add it into the chain
func (c *Consensus) MinedNewBlock(b *ngtypes.Block) {
	c.Lock()
	defer c.Unlock()

	// check whether block has the correct nonce
	err := b.CheckError()
	if err != nil {
		log.Warning("Malformed block mined:", err)
		return
	}

	prevBlock, err := c.Chain.GetBlockByHash(b.Header.PrevBlockHash)
	if err != nil {
		log.Error("cannot find the prevBlock for new block, rejected:", err)
		return
	}

	if prevBlock == nil {
		log.Warning("Malformed block mined: PrevBlockHash")
		return
	}

	prevVault, err := c.Chain.GetVaultByHash(b.Header.PrevVaultHash)
	if err != nil {
		log.Error("cannot find the prevVault for new block, rejected:", err)
		return
	}
	if prevVault == nil {
		log.Warning("Malformed block mined: PrevVaultHash")
		return
	}

	hash, _ := b.CalculateHash()
	log.Infof("Mined a new Block: %x@%d", hash, b.GetHeight())

	// TODO: vault should be generated when made the template
	if b.Header.IsHead() {
		err := c.SheetManager.HandleVault(prevVault)
		if err != nil {
			log.Error(err)
		}
	}

	err = c.Chain.MinedNewBlock(b)
	if err != nil {
		log.Warning(err)
		return
	}

	err = c.SheetManager.HandleTxs(b.Transactions...)
	if err != nil {
		log.Warning(err)
		return
	}

	if b.Header.IsTail() {
		currentVault := c.GenNewVault(b.Header.Height/ngtypes.BlockCheckRound, b.Header.PrevVaultHash)
		err := c.Chain.MinedNewVault(currentVault)
		if err != nil {
			panic(err)
		}
	}
}

// GenNewVault is called when the reached a checkpoint, then generate a
func (c *Consensus) GenNewVault(prevVaultHeight uint64, prevVaultHash []byte) *ngtypes.Vault {
	// Make this value can be DIY by users
	accountNumber := utils.RandUint64()
	log.Infof("New account: %d", accountNumber)

	sheet, err := c.SheetManager.GenerateNewSheet()
	if err != nil {
		log.Error(err)
		return nil
	}
	newVault := ngtypes.NewVault(prevVaultHeight, prevVaultHash, sheet)
	hash, _ := newVault.CalculateHash()
	log.Infof("Generated a new Vault@%d, %x", newVault.GetHeight(), hash)
	return newVault
}
