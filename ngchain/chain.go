package ngchain

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("ngchain")

const (
	latestHeightTag = "height"
	latestHashTag   = "hash"
)

var (
	blockPrefix = []byte("blk")
)

// Chain managers a badger DB, which stores vaults and blocks and some helper tags for managing
type Chain struct {
	db *badger.DB

	MinedBlockToP2PCh    chan *ngtypes.Block
	MinedBlockToTxPoolCh chan *ngtypes.Block
}

// NewChain will return a chain, but no initialization
func NewChain(db *badger.DB) *Chain {
	chain := &Chain{
		db: db,

		MinedBlockToP2PCh:    make(chan *ngtypes.Block),
		MinedBlockToTxPoolCh: make(chan *ngtypes.Block),
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

func (c *Chain) GetLatestBlockHash() []byte {
	hash, err := c.GetLatestBlock().CalculateHash()
	if err != nil {
		log.Error(err)
		return nil
	}

	return hash
}

func (c *Chain) GetLatestBlockHeight() uint64 {
	var latestHeight uint64
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(blockPrefix, latestHeightTag...))
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

		err = utils.Proto.Unmarshal(raw, block)
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
	if bytes.Equal(hash, ngtypes.GetGenesisBlockHash()) {
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

		err = utils.Proto.Unmarshal(raw, block)
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
			err = utils.Proto.Unmarshal(raw, block)
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
			err = utils.Proto.Unmarshal(raw, block)
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
