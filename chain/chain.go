package chain

import (
	"errors"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"go.etcd.io/bbolt"
)

var log = logging.MustGetLogger("chain")

// chain order vault0-block0-block1...-block6-vault2-block7...
type Chain struct {
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
	memItem, err := c.mem.GetLatestBlock()
	if err != nil {
		log.Error(err)
	}

	dbItem, err := c.db.GetLatestBlock()
	if err != nil {
		log.Error(err)
	}

	if dbItem == nil {
		return memItem
	}

	if memItem == nil {
		return dbItem
	}

	if dbItem.GetHeight() >= memItem.GetHeight() {
		return dbItem
	} else {
		return memItem
	}
}

func (c *Chain) GetLatestVault() *ngtypes.Vault {
	memItem, err := c.mem.GetLatestVault()
	if err != nil {
		log.Error(err)
	}

	dbItem, err := c.db.GetLatestVault()
	if err != nil {
		log.Error(err)
	}

	if dbItem == nil {
		return memItem
	}

	if memItem == nil {
		return dbItem
	}

	if dbItem.GetHeight() >= memItem.GetHeight() {
		return dbItem
	} else {
		return memItem
	}
}

func (c *Chain) GetLatestBlockHash() []byte {
	hash := c.mem.GetLatestBlockHash()
	if hash != nil {
		return hash
	}

	hash, err := c.db.GetLatestBlockHash()
	if err != nil {
		log.Error(err)
	}

	return hash
}

func (c *Chain) GetLatestVaultHash() []byte {
	hash := c.mem.GetLatestVaultHash()
	if hash != nil {
		return hash
	}

	hash, err := c.db.GetLatestVaultHash()
	if err != nil {
		log.Error(err)
	}

	return hash
}

func (c *Chain) GetLatestBlockHeight() uint64 {
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
	if block.GetHeight()%(ngtypes.BlockCheckRound*ngtypes.VaultCheckRound) == 0 &&
		c.GetLatestBlockHeight() < block.Header.Height &&
		len(c.mem.BlockHeightMap) >= ngtypes.BlockCheckRound {

		log.Info("dumping blocks from mem to db")
		prevBlock := c.GetBlockByHash(block.GetPrevHash())
		items := c.mem.ExportLongestChain(prevBlock, ngtypes.BlockCheckRound*ngtypes.VaultCheckRound)
		if len(items) > 0 {
			err := c.db.PutChain(items...)
			if err != nil {
				log.Info("failed to put items:", err)
				return err
			}
			//bc.mem.ReleaseMap(block)
		} else {
			return errors.New("chain malformed")
		}
	}

	err := c.mem.PutItems(block)
	if err != nil {
		return err
	}

	return nil
}

// PutVault puts an vault into db
func (c *Chain) PutVault(vault *ngtypes.Vault) error {
	err := c.mem.PutItems(vault)
	if err != nil {
		return err
	}

	return nil
}

func (c *Chain) GetBlockByHeight(height uint64) *ngtypes.Block {
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
	item, err := c.mem.GetBlockByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if item != nil {
		return item
	}

	item, err = c.db.GetBlockByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if item != nil {
		return item
	}

	return nil
}

func (c *Chain) GetVaultByHeight(height uint64) *ngtypes.Vault {
	item, err := c.mem.GetVaultByHeight(height)
	if err != nil {
		log.Error(err)
	}
	if item != nil {
		return item
	}

	item, err = c.db.GetVaultByHeight(height)
	if err != nil {
		log.Error(err)
	}
	if item != nil {
		return item
	}

	return nil
}

func (c *Chain) GetVaultByHash(hash []byte) *ngtypes.Vault {
	item, err := c.mem.GetVaultByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if item != nil {
		return item
	}

	item, err = c.db.GetVaultByHash(hash)
	if err != nil {
		log.Error(err)
	}
	if item != nil {
		return item
	}

	return nil
}

func (c *Chain) DumpAllBlocksByHeight() map[uint64]*ngtypes.Block {
	return c.db.DumpAllBlocksByHeight()
}

func (c *Chain) DumpAllVaultsByHeight() map[uint64]*ngtypes.Vault {
	return c.db.DumpAllVaultsByHeight()
}

func (c *Chain) DumpAllByHash(withBlocks bool, withVaults bool) map[string]Item {
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
