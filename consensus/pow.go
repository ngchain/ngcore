// act as the main of the pow consensus
// pow -
package consensus

import (
	"bytes"
	"crypto/elliptic"
	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
	"github.com/whyrusleeping/go-logging"
	"runtime"
)

var log = logging.MustGetLogger("consensus")

// the main of consensus, shouldn't be shut down
func (c *Consensus) InitPoW(newBlockCh chan *ngtypes.Block) {
	if c.mining {
		log.Info("Start mining")
		// TODO: add mining cpu number flag
		c.miner = miner.NewMiner(runtime.NumCPU()/2, newBlockCh)
		c.miner.Start(c.GetBlockTemplate())
	}

	go func() {
		for {
			select {
			case b := <-newBlockCh:
				c.MinedNewBlock(b)

				if c.mining {
					c.miner.Start(c.GetBlockTemplate())
				}
			}
		}
	}()
}

func (c *Consensus) StopMining() {
	log.Info("mining stopping")
	c.miner.Stop()
}

func (c *Consensus) ResumeMining() {
	log.Info("mining resuming")
	c.miner.Start(c.GetBlockTemplate())
}

func (c *Consensus) GetBlockTemplate() *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	currentBlock := c.Chain.GetLatestBlock()
	currentBlockHash, _ := currentBlock.CalculateHash()
	if bytes.Compare(currentBlockHash, c.Chain.GetLatestBlockHash()) != 0 {
		panic("")
	}
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

	Gen := c.CreateGeneration(c.privateKey, newBlockHeight, currentBlockHash, currentVaultHash, extraData)
	txsWithGen := append([]*ngtypes.Transaction{Gen}, c.TxPool.GetPackTxs(MaxTxsSize)...)
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
		err := c.SheetManager.ApplyVault(prevVault)
		if err != nil {
			log.Error(err)
		}
	}

	err = c.Chain.MinedNewBlock(b) // TODO: chain should verify the block
	if err != nil {
		log.Warning(err)
		return
	}

	c.SheetManager.ApplyBlockTxs(b)

	if b.Header.IsTail() {
		currentVault := c.GenNewVault(b.Header.Height/ngtypes.BlockCheckRound, b.Header.PrevVaultHash)
		err := c.Chain.MinedNewVault(currentVault)
		if err != nil {
			panic(err)
		}
	}

	c.template = nil
}

// GenNewVault is called when the reached a checkpoint, then generate a
func (c *Consensus) GenNewVault(prevVaultHeight uint64, prevVaultHash []byte) *ngtypes.Vault {
	// Make this value can be DIY by users
	accountNumber := utils.RandUint64()
	log.Infof("New account: %d", accountNumber)

	ownerKey := elliptic.Marshal(elliptic.P256(), c.privateKey.PublicKey.X, c.privateKey.PublicKey.Y)

	newVault := ngtypes.NewVault(accountNumber, ownerKey, prevVaultHeight, prevVaultHash, c.SheetManager.GenerateSheet())
	hash, _ := newVault.CalculateHash()
	log.Infof("Generated a new Vault@%d, %x", newVault.GetHeight(), hash)
	return newVault
}
