package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// GetLatestBlock will return the latest Block in DB.
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

	if err := c.db.View(func(txn *badger.Txn) error {
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
	}); err != nil {
		return 0
	}

	return latestHeight
}

func (c *Chain) GetLatestCheckpointHash() []byte {
	cp, err := c.GetLatestCheckpoint()
	if err != nil {
		log.Errorf("failed GetLatestCheckpointHash:", err)

		return nil
	}

	hash, _ := cp.CalculateHash()
	return hash
}

func (c *Chain) GetLatestCheckpoint() (*ngtypes.Block, error) {
	b := c.GetLatestBlock()
	if b.IsGenesis() {
		return b, nil
	}

	if err := c.db.View(func(txn *badger.Txn) error {
		var raw []byte
		for {
			item, err := txn.Get(append(blockPrefix, b.GetPrevHash()...))
			if err != nil {
				return err
			}

			raw, err = item.ValueCopy(nil)
			if err != nil {
				return err
			}

			err = utils.Proto.Unmarshal(raw, b)
			if err != nil {
				return err
			}

			if b.IsHead() {
				return nil
			}
		}
	}); err != nil {
		return nil, err
	}

	return b, nil
}

func (c *Chain) GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	if height == 0 {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}

	if err := c.db.View(func(txn *badger.Txn) error {
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
	}); err != nil {
		return nil, err
	}

	return block, nil
}

func (c *Chain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	if bytes.Equal(hash, ngtypes.GetGenesisBlockHash()) {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}

	if err := c.db.View(func(txn *badger.Txn) error {
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
	}); err != nil {
		return nil, err
	}

	return block, nil
}

func (c *Chain) GetOriginBlock() *ngtypes.Block {
	return ngtypes.GetGenesisBlock() // TODO
}
