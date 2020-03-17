package chain

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"go.etcd.io/bbolt"
	"sync"
)

var log = logging.MustGetLogger("chain")

// chain order vault0-block0-block1...-block6-vault2-block7...
type Chain struct {
	sync.RWMutex
	db  *storageChain
	mem *MemChain
}

func NewChain(db *bbolt.DB) *Chain {
	sc := NewStorageChain(db)

	chain := &Chain{
		db:  sc,
		mem: NewMemChain(),
	}

	return chain
}

func (c *Chain) InitWithGenesis() {
	c.db.Init(ngtypes.GetGenesisBlock(), ngtypes.GetGenesisVault())
	c.mem.Init(ngtypes.GetGenesisBlock(), ngtypes.GetGenesisVault())
}

func (c *Chain) GetLatestBlock() *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	return c.GetBlockByHeight(c.GetLatestBlockHeight())
}

func (c *Chain) GetLatestVault() *ngtypes.Vault {
	c.RLock()
	defer c.RUnlock()

	return c.GetVaultByHeight(c.GetLatestVaultHeight())
}

func (c *Chain) GetLatestBlockHash() []byte {
	c.RLock()
	defer c.RUnlock()

	hash, err := c.GetLatestBlock().CalculateHash()
	if err != nil {
		log.Error(err)
		return nil
	}

	return hash
}

func (c *Chain) GetLatestVaultHash() []byte {
	c.RLock()
	defer c.RUnlock()

	hash, err := c.GetLatestVault().CalculateHash()
	if err != nil {
		log.Error(err)
		return nil
	}

	return hash
}

func (c *Chain) GetLatestBlockHeight() uint64 {
	c.RLock()
	defer c.RUnlock()

	height := c.mem.GetLatestBlockHeight()
	if height != 0 {
		return height
	}

	height, err := c.db.GetLatestBlockHeight()
	if err != nil {
		log.Error(err)
	}

	return height
}

func (c *Chain) GetLatestVaultHeight() uint64 {
	c.RLock()
	defer c.RUnlock()

	height := c.mem.GetLatestVaultHeight()
	if height != 0 {
		return height
	}

	height, err := c.db.GetLatestVaultHeight()
	if err != nil {
		log.Error(err)
	}

	return height
}

func (c *Chain) PutBlock(block *ngtypes.Block) error {
	c.Lock()
	defer c.Unlock()

	if block.GetHeight()%(ngtypes.BlockCheckRound*ngtypes.VaultCheckRound) == 0 &&
		//c.GetLatestBlockHeight() < block.Header.Height &&
		len(c.mem.BlockHeightMap) >= ngtypes.BlockCheckRound {
		go func() {
			log.Info("dumping blocks from mem to db")
			prevBlock := c.GetBlockByHash(block.GetPrevHash())
			items := c.mem.ExportLongestChain(prevBlock, ngtypes.BlockCheckRound*ngtypes.VaultCheckRound)
			if len(items) > 0 {
				err := c.db.PutChain(items...)
				if err != nil {
					log.Info("failed to put items:", err)
				}
			}
		}()
	}

	err := c.mem.PutBlocks(block)
	if err != nil {
		return err
	}

	return nil
}

// PutVault puts an vault into db
func (c *Chain) PutVault(vault *ngtypes.Vault) error {
	c.Lock()
	defer c.Unlock()

	err := c.mem.PutVaults(vault)
	if err != nil {
		return err
	}

	return nil
}

func (c *Chain) GetBlockByHeight(height uint64) *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	block, err := c.mem.GetBlockByHeight(height)
	if err != nil {
		log.Error(err)
	}
	if block != nil {
		return block
	}

	block, err = c.db.GetBlockByHeight(height)
	if err != nil {
		log.Error(err)
	}
	if block != nil {
		return block
	}

	return nil
}

func (c *Chain) GetBlockByHash(hash []byte) *ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	block, err := c.mem.GetBlockByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if block != nil {
		return block
	}

	block, err = c.db.GetBlockByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if block != nil {
		return block
	}

	return nil
}

func (c *Chain) GetVaultByHeight(height uint64) *ngtypes.Vault {
	c.RLock()
	defer c.RUnlock()

	vault, err := c.mem.GetVaultByHeight(height)
	if err != nil {
		log.Error(err)
	}
	if vault != nil {
		return vault
	}

	vault, err = c.db.GetVaultByHeight(height)
	if err != nil {
		log.Error(err)
	}
	if vault != nil {
		return vault
	}

	return nil
}

func (c *Chain) GetVaultByHash(hash []byte) *ngtypes.Vault {
	c.RLock()
	defer c.RUnlock()

	vault, err := c.mem.GetVaultByHash(hash)
	if err != nil {
		log.Error(err)
		log.Info(c.mem.VaultHashMap)
	}
	if vault != nil {
		return vault
	}

	vault, err = c.db.GetVaultByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if vault != nil {
		return vault
	}

	return nil
}

func (c *Chain) DumpAllBlocksByHeight() map[uint64]*ngtypes.Block {
	c.RLock()
	defer c.RUnlock()

	return c.db.DumpAllBlocksByHeight()
}

func (c *Chain) DumpAllVaultsByHeight() map[uint64]*ngtypes.Vault {
	c.RLock()
	defer c.RUnlock()

	c.RLock()
	defer c.RUnlock()

	return c.db.DumpAllVaultsByHeight()
}

func (c *Chain) DumpAllByHash(withBlocks bool, withVaults bool) map[string]Item {
	c.RLock()
	defer c.RUnlock()

	kv := make(map[string]Item)
	if withBlocks {
		all := c.db.DumpAllBlocksByHash()
		for k, v := range all {
			kv[k] = v
		}
	}
	if withVaults {
		all := c.db.DumpAllVaultsByHash()
		for k, v := range all {
			kv[k] = v
		}
	}
	return kv
}
