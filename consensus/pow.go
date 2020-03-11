// act as the main of the pow consensus
// pow -
package consensus

import (
	"crypto/ecdsa"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/txpool"
	"runtime"
	"time"

	"github.com/ngin-network/ngcore/chain"
	"github.com/whyrusleeping/go-logging"
	"golang.org/x/crypto/sha3"

	"math/big"
)

var log = logging.MustGetLogger("consensus")

// the pow
type Consensus struct {
	template     *ngtypes.Block
	SheetManager *sheetManager.SheetManager

	privateKey *ecdsa.PrivateKey
	BlockChain *chain.BlockChain
	VaultChain *chain.VaultChain

	CurrentBlock *ngtypes.Block
	CurrentVault *ngtypes.Vault

	TxPool *txpool.TxPool
}

func (c *Consensus) PoW(mining bool) {
	var m *Miner
	stopCh := make(chan struct{})
	foundCh := make(chan *ngtypes.Block)
	if mining {
		log.Info("Start mining")
		// TODO: add mining cpu number flag
		m = NewMiner(runtime.NumCPU()/2, stopCh, foundCh, c.GetBlockTemplate())
	}

	updateCh := time.Tick(2 * time.Second)

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

		}
	}
}

func (c *Consensus) GetBlockTemplate() (newUnsealingBlock *ngtypes.Block) {
	currentBlock := c.BlockChain.GetLatestBlock()
	currentVault := c.VaultChain.GetLatestVault()

	newTarget := c.getNextTarget(currentBlock, currentVault)
	newBlockHeight := currentBlock.Header.Height + 1

	currentBlockHash := c.BlockChain.GetLatestBlockHash()
	currentVaultHash := c.VaultChain.GetLatestVaultHash()

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
	err = c.BlockChain.PutBlock(b) // chain should verify again
	if err != nil {
		log.Warning(err)
		return
	}

	if b.Header.IsCheckpoint() {
		// TODO: If broadcast block only, without Vault?
		// p2p broadcast block(with V hash) -> Other Node will found it forgot its new V and reject it.
		go func() {
			v := c.GenNewVault(b)
			c.ApplyNewVault(v)
		}()
	}

	// block reward to frozen room
	go c.SheetManager.ApplyBlockTxs(b)

	c.CurrentBlock = c.BlockChain.GetLatestBlock()
}

// GenNewVault is called when the reached a checkpoint, then generate a
func (c *Consensus) GenNewVault(hookBlock *ngtypes.Block) *ngtypes.Vault {
	log.Infof("Mined a new Vault on: %d", hookBlock.Header.Height)

	// Make this value can be DIY by users
	accountNumber := GetNewAccountIdByHookBlock(hookBlock)
	log.Infof("New account: %d", accountNumber)

	return ngtypes.NewVault(accountNumber, c.VaultChain.GetLatestVault(), hookBlock, c.SheetManager.GenerateSheet())
}

//
func (c *Consensus) ApplyNewVault(v *ngtypes.Vault) {
	log.Info("applying new vault:", v)
	err := c.VaultChain.PutVault(v)
	if err != nil && err != chain.ErrItemHashInSameHeight {
		log.Panic(err)
	}

	// handle with the new vault
	// register the new Account in txpool
	log.Info(v)
	err = c.SheetManager.ApplyVault(v)
	if err != nil {
		log.Panic(err)
	}

	// update
	c.CurrentVault = c.VaultChain.GetLatestVault()
}

func GetNewAccountIdByHookBlock(hookBlock *ngtypes.Block) uint64 {
	//TODO: Make this value can be DIY by users
	b := sha3.Sum256(hookBlock.HeaderHash)
	return new(big.Int).SetBytes(b[:]).Uint64()
}
