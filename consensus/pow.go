// act as the main of the pow consensus
// pow -
package consensus

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"golang.org/x/crypto/sha3"
	"runtime"
	"time"

	"math/big"
)

var log = logging.MustGetLogger("consensus")

func (c *Consensus) PoW(mining bool, stopCh chan struct{}) {
	var m *Miner
	foundCh := make(chan *ngtypes.Block)
	if mining {
		log.Info("Start mining")
		// TODO: add mining cpu number flag
		m = NewMiner(runtime.NumCPU()/2, stopCh, foundCh, c.GetBlockTemplate())
	}

	updateCh := time.Tick(1 * time.Second)

	for {
		select {
		case b := <-foundCh:
			c.MinedNewBlock(b)
			// assign new work
			// assign the reward/balance to self
			// if mined the block(not checkpoint) with the pubkey without account -> frozen, cannot tx until get an account

			if mining {
				go m.SetJob(c.GetBlockTemplate())
			}
		case <-updateCh:
			if mining {
				go m.SetJob(c.GetBlockTemplate())
			}
		case <-stopCh:
			return

		}
	}
}

func (c *Consensus) GetBlockTemplate() (newUnsealingBlock *ngtypes.Block) {
	currentBlock := c.Chain.GetLatestBlock()
	currentVault := c.Chain.GetLatestVault()

	newTarget := c.getNextTarget(currentBlock, currentVault)
	newBlockHeight := currentBlock.Header.Height + 1

	currentBlockHash := c.Chain.GetLatestBlockHash()
	currentVaultHash := c.Chain.GetLatestVaultHash()

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
	log.Infof("Mined a new Block: %x", hash)

	// TODO: vault should be generated when made the template
	if b.Header.IsCheckpoint() {
		v := c.GenNewVault(b)
		c.ApplyNewVault(v)
	}

	err = c.Chain.PutBlock(b) // chain should verify again
	if err != nil {
		log.Warning(err)
		return
	}

	c.SheetManager.ApplyBlockTxs(b)

	//c.CurrentBlock = c.BlockChain.GetLatestBlock()
}

// GenNewVault is called when the reached a checkpoint, then generate a
func (c *Consensus) GenNewVault(hookBlock *ngtypes.Block) *ngtypes.Vault {
	log.Infof("Mined a new Vault on: %d", hookBlock.Header.Height)

	// Make this value can be DIY by users
	accountNumber := GetNewAccountIdByHookBlock(hookBlock)
	log.Infof("New account: %d", accountNumber)

	return ngtypes.NewVault(accountNumber, hookBlock, c.SheetManager.GenerateSheet())
}

//
func (c *Consensus) ApplyNewVault(v *ngtypes.Vault) {
	log.Infof("applying new vault @ %d", v.Height)
	err := c.Chain.PutVault(v)
	if err != nil {
		log.Error(err)
	}

	// handle with the new vault
	// register the new Account in txpool
	err = c.SheetManager.ApplyVault(v)
	if err != nil {
		log.Panic(err)
	}
}

func GetNewAccountIdByHookBlock(hookBlock *ngtypes.Block) uint64 {
	//TODO: Make this value can be DIY by users
	b := sha3.Sum256(hookBlock.HeaderHash)
	return new(big.Int).SetBytes(b[:]).Uint64()
}
