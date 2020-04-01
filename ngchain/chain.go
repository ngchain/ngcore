package ngchain

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
	"github.com/whyrusleeping/go-logging"
)

var log = logging.MustGetLogger("chain")

var (
	vaultPrefix = []byte("vlt")
	blockPrefix = []byte("blk")
)

// chain managers a badger DB, which stores vaults and blocks and some helper tags for managing
type Chain struct {
	db *badger.DB

	MinedBlockToP2PCh    chan *ngtypes.Block
	MinedBlockToTxPoolCh chan *ngtypes.Block
	NewVaultToTxPoolCh   chan *ngtypes.Vault
}

// NewChain will return a chain, but no initialization
func NewChain(db *badger.DB) *Chain {
	chain := &Chain{
		db: db,

		MinedBlockToP2PCh:    make(chan *ngtypes.Block),
		MinedBlockToTxPoolCh: make(chan *ngtypes.Block),
		NewVaultToTxPoolCh:   make(chan *ngtypes.Vault),
	}

	return chain
}

// GetLatestBlock will return the latest Block in DB
func (c *Chain) GetLatestBlock() *ngtypes.Block {
	height := c.GetLatestBlockHeight()
	block, err := c.GetBlockByHeight(height)
	if err != nil {
		log.Error(err)
	}

	return block
}

// GetLatestVault will return the latest Vault in DB
func (c *Chain) GetLatestVault() *ngtypes.Vault {
	height := c.GetLatestVaultHeight()
	vault, err := c.GetVaultByHeight(height)
	if err != nil {
		log.Error(err)
	}

	return vault
}

func (c *Chain) GetLatestBlockHash() []byte {
	hash, err := c.GetLatestBlock().CalculateHash()
	if err != nil {
		log.Error(err)
		return nil
	}

	return hash
}

func (c *Chain) GetLatestVaultHash() []byte {
	hash, err := c.GetLatestVault().CalculateHash()
	if err != nil {
		log.Error(err)
		return nil
	}

	return hash
}

func (c *Chain) GetLatestBlockHeight() uint64 {
	var latestHeight uint64
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(blockPrefix, LatestHeightTag...))
		if err != nil {
			return err
		}
		err = item.Value(func(height []byte) error {
			latestHeight = binary.LittleEndian.Uint64(height)
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return 0
	}

	return latestHeight
}

func (c *Chain) GetLatestVaultHeight() uint64 {
	var latestHeight uint64
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(vaultPrefix, LatestHeightTag...))
		if err != nil {
			return err
		}
		err = item.Value(func(height []byte) error {
			latestHeight = binary.LittleEndian.Uint64(height)
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return 0
	}

	return latestHeight
}

func (c *Chain) GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	if height == 0 {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(blockPrefix, utils.PackUint64LE(height)...))
		if err != nil {
			return err
		}
		hash, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if hash == nil {
			return fmt.Errorf("no such block in height")
		}
		item, err = txn.Get(append(blockPrefix, hash...))
		if err != nil {
			return err
		}
		raw, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if raw == nil {
			return fmt.Errorf("no such block in hash")
		}

		err = block.Unmarshal(raw)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (c *Chain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	if !bytes.Equal(hash, ngtypes.GenesisBlockHash) {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(blockPrefix, hash...))
		if err != nil {
			return err
		}
		raw, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if raw == nil {
			return fmt.Errorf("no such block in hash")
		}

		err = block.Unmarshal(raw)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (c *Chain) GetVaultByHeight(height uint64) (*ngtypes.Vault, error) {
	if height == 0 {
		return ngtypes.GetGenesisVault(), nil
	}

	var vault = &ngtypes.Vault{}
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(vaultPrefix, utils.PackUint64LE(height)...))
		if err != nil {
			return err
		}
		hash, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if hash == nil {
			return fmt.Errorf("no such vault in height")
		}
		item, err = txn.Get(append(vaultPrefix, hash...))
		if err != nil {
			return err
		}
		raw, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if raw == nil {
			return fmt.Errorf("no such vault in hash")
		}

		err = vault.Unmarshal(raw)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return vault, nil
}

func (c *Chain) GetVaultByHash(hash []byte) (*ngtypes.Vault, error) {
	if bytes.Equal(hash, ngtypes.GenesisVaultHash) {
		return ngtypes.GetGenesisVault(), nil
	}

	var vault = &ngtypes.Vault{}
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(vaultPrefix, hash...))
		if err != nil {
			return err
		}
		raw, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if raw == nil {
			return fmt.Errorf("no such vault in hash")
		}

		err = vault.Unmarshal(raw)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return vault, nil
}

func (c *Chain) DumpAllBlocksByHeight() map[uint64]*ngtypes.Block {
	table := make(map[uint64]*ngtypes.Block)
	err := c.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(blockPrefix); it.ValidForPrefix(blockPrefix) && len(it.Item().Key()) == 11; it.Next() {
			height := binary.LittleEndian.Uint64(it.Item().Key()[3:11])
			hash, err := it.Item().ValueCopy(nil)
			if err != nil {
				return err
			}
			i, err := txn.Get(append(blockPrefix, hash...))
			if err != nil {
				return err
			}
			raw, err := i.ValueCopy(nil)
			if err != nil {
				return err
			}
			var block = &ngtypes.Block{}
			err = block.Unmarshal(raw)
			if err != nil {
				return err
			}
			table[height] = block
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return table
}

func (c *Chain) DumpAllBlocksByHash() map[string]*ngtypes.Block {
	table := make(map[string]*ngtypes.Block)
	err := c.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(blockPrefix); it.ValidForPrefix(blockPrefix) && len(it.Item().Key()) == 35; it.Next() {
			hash := it.Item().Key()[3:35]
			i, err := txn.Get(append(blockPrefix, hash...))
			if err != nil {
				return err
			}
			raw, err := i.ValueCopy(nil)
			if err != nil {
				return err
			}
			var block = &ngtypes.Block{}
			err = block.Unmarshal(raw)
			if err != nil {
				return err
			}
			table[hex.EncodeToString(hash)] = block
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return table
}

func (c *Chain) DumpAllVaultsByHeight() map[uint64]*ngtypes.Vault {
	table := make(map[uint64]*ngtypes.Vault)
	err := c.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(vaultPrefix); it.ValidForPrefix(vaultPrefix) && len(it.Item().Key()) == 11; it.Next() {
			height := binary.LittleEndian.Uint64(it.Item().Key()[3:11])
			hash, err := it.Item().ValueCopy(nil)
			if err != nil {
				return err
			}
			i, err := txn.Get(append(vaultPrefix, hash...))
			if err != nil {
				return err
			}
			raw, err := i.ValueCopy(nil)
			if err != nil {
				return err
			}
			var vault = &ngtypes.Vault{}
			err = vault.Unmarshal(raw)
			if err != nil {
				return err
			}
			table[height] = vault
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return table
}

func (c *Chain) DumpAllVaultsByHash() map[string]*ngtypes.Vault {
	table := make(map[string]*ngtypes.Vault)
	err := c.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(vaultPrefix); it.ValidForPrefix(vaultPrefix) && len(it.Item().Key()) == 35; it.Next() {
			hash := it.Item().Key()[3:35]
			i, err := txn.Get(append(vaultPrefix, hash...))
			if err != nil {
				return err
			}
			raw, err := i.ValueCopy(nil)
			if err != nil {
				return err
			}
			var vault = &ngtypes.Vault{}
			err = vault.Unmarshal(raw)
			if err != nil {
				return err
			}
			table[hex.EncodeToString(hash)] = vault
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return table
}

func (c *Chain) DumpAllByHash(withBlocks bool, withVaults bool) map[string]Item {
	kv := make(map[string]Item)
	if withBlocks {
		all := c.DumpAllBlocksByHash()
		for k, v := range all {
			kv[k] = v
		}
	}

	if withVaults {
		all := c.DumpAllVaultsByHash()
		for k, v := range all {
			kv[k] = v
		}
	}
	return kv
}
