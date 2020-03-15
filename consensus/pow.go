// act as the main of the pow consensus
// pow -
package consensus

import (
	"crypto/elliptic"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/utils"
	"github.com/whyrusleeping/go-logging"
	"runtime"
	"time"
)

var log = logging.MustGetLogger("consensus")

// the main of consensus, shouldn't be shut down
func (c *Consensus) InitPoW() {
	foundCh := make(chan *ngtypes.Block)
	if c.mining {
		log.Info("Start mining")
		// TODO: add mining cpu number flag
		c.miner = NewMiner(runtime.NumCPU()/2, foundCh)
		c.miner.start(c.GetBlockTemplate())
	}

	updateCh := time.Tick(1 * time.Second)

	for {
		select {
		case b := <-foundCh:
			c.MinedNewBlock(b)
			// assign new work
			// assign the reward/balance to self
			// if mined the block(not checkpoint) with the pubkey without account -> frozen, cannot tx until get an account

			if c.mining {
				go c.miner.setJob(c.GetBlockTemplate())
			}
		case <-updateCh:
			if c.mining {
				go c.miner.setJob(c.GetBlockTemplate())
			}
		}
	}
}

func (c *Consensus) StopMining() {
	log.Info("mining stopping")
	c.miner.abortCh <- struct{}{}
}

func (c *Consensus) ResumeMining() {
	log.Info("mining resuming")
	c.miner.start(c.GetBlockTemplate())
}

func (c *Consensus) GetBlockTemplate() (newUnsealingBlock *ngtypes.Block) {
	if c.template != nil {
		return c.template
	}

	currentBlock := c.Chain.GetLatestBlock()
	currentBlockHash, _ := currentBlock.CalculateHash()
	newBlockHeight := currentBlock.Header.Height + 1

	currentVault := c.Chain.GetLatestVault()
	currentVaultHash, _ := currentVault.CalculateHash()

	newTarget := c.getNextTarget(currentBlock, currentVault)

	newBareBlock := ngtypes.NewBareBlock(
		newBlockHeight,
		currentBlockHash,
		currentVaultHash,
		newTarget,
	)

	extraData := []byte("ngCore")

	// TODO: add pending Transactions to the template
	Gen := c.CreateGeneration(c.privateKey, newBlockHeight, currentBlockHash, currentVaultHash, extraData)
	txsWithGen := append([]*ngtypes.Transaction{Gen}, c.TxPool.GetPackTxs(MaxTxsSize)...)
	newUnsealingBlock, err := newBareBlock.ToUnsealing(txsWithGen)
	if err != nil {
		log.Error(err)
	}

	c.template = newUnsealingBlock

	return newUnsealingBlock
}

// mined new block and need to add it into the chain
func (c *Consensus) MinedNewBlock(b *ngtypes.Block) {
	// check whether block has the correct nonce
	err := b.CheckError()
	if err != nil {
		log.Warning("Malformed block mined:", err)
		return
	}

	// add block to the chain

	hash, _ := b.CalculateHash()
	log.Infof("Mined a new Block: %x@%d", hash, b.GetHeight())

	// TODO: vault should be generated when made the template
	if b.Header.IsHead() {
		v := c.Chain.GetVaultByHash(b.Header.PrevVaultHash)
		err := c.SheetManager.ApplyVault(v)
		if err != nil {
			log.Panic(err)
		}
	}

	if b.Header.IsTail() {
		currentVault := c.GenNewVault(c.Chain.GetLatestVaultHeight(), c.Chain.GetLatestVaultHash())
		c.Chain.PutVault(currentVault)
	}

	err = c.Chain.PutBlock(b) // chain should verify again
	if err != nil {
		log.Warning(err)
		return
	}

	c.SheetManager.ApplyBlockTxs(b)

	c.template = nil
}

// GenNewVault is called when the reached a checkpoint, then generate a
func (c *Consensus) GenNewVault(prevVaultHeight uint64, prevVaultHash []byte) *ngtypes.Vault {
	log.Infof("Mined a new Vault@%d", prevVaultHeight)

	// Make this value can be DIY by users
	accountNumber := utils.RandUint64()
	log.Infof("New account: %d", accountNumber)

	ownerKey := elliptic.Marshal(elliptic.P256(), c.privateKey.PublicKey.X, c.privateKey.PublicKey.Y)
	return ngtypes.NewVault(accountNumber, ownerKey, prevVaultHeight, prevVaultHash, c.SheetManager.GenerateSheet())
}
