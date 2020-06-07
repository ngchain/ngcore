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

// GetLatestBlockHash will fetch the latest block from chain and then calc its hash
func (c *Chain) GetLatestBlockHash() []byte {
	return c.GetLatestBlock().Hash()
}

// GetLatestBlockHeight will fetch the latest block from chain and then return its height
func (c *Chain) GetLatestBlockHeight() uint64 {
	var latestHeight uint64

	if err := c.db.View(func(txn *badger.Txn) error {
		key := append(blockPrefix, latestHeightTag...)
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("failed to get item by key %x: %s", key, err)
		}
		raw, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("no such height in latestTag: %s", err)
		}

		latestHeight = binary.LittleEndian.Uint64(raw)

		return nil
	}); err != nil {
		return 0
	}

	return latestHeight
}

// GetLatestCheckpointHash returns the hash of latest checkpoint
func (c *Chain) GetLatestCheckpointHash() []byte {
	cp := c.GetLatestCheckpoint()
	return cp.Hash()
}

// GetLatestCheckpoint returns the latest checkpoint block
func (c *Chain) GetLatestCheckpoint() *ngtypes.Block {
	b := c.GetLatestBlock()
	if b.IsGenesis() {
		return b
	}

	if err := c.db.View(func(txn *badger.Txn) error {
		var raw []byte
		for {
			hash := b.GetPrevHash()
			key := append(blockPrefix, hash...)
			item, err := txn.Get(key)
			if err != nil {
				return fmt.Errorf("failed to get item by key %x: %s", key, err)
			}
			raw, err = item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("no such block in hash %x: %s", hash, err)
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
		log.Errorf("error when getting latest checkpoint, maybe chain is broken, please resync: %s", err)
	}

	return b
}

// GetBlockByHeight returns a block by height inputed
func (c *Chain) GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	if height == 0 {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}

	if err := c.db.View(func(txn *badger.Txn) error {
		key := append(blockPrefix, utils.PackUint64LE(height)...)
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("failed to get item by key %x: %s", key, err)
		}
		hash, err := item.ValueCopy(nil)
		if err != nil || hash == nil {
			return fmt.Errorf("no such block in height %d: %s", height, err)
		}
		key = append(blockPrefix, hash...)
		item, err = txn.Get(key)
		if err != nil {
			return fmt.Errorf("failed to get item by key %x: %s", key, err)
		}
		raw, err := item.ValueCopy(nil)
		if err != nil || raw == nil {
			return fmt.Errorf("no such block in hash %x: %s", hash, err)
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

// GetBlockByHash returns a block by hash inputed
func (c *Chain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	if bytes.Equal(hash, ngtypes.GetGenesisBlockHash()) {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}

	if err := c.db.View(func(txn *badger.Txn) error {
		key := append(blockPrefix, hash...)
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("failed to get item by key %x: %s", key, err)
		}
		raw, err := item.ValueCopy(nil)
		if err != nil || raw == nil {
			return fmt.Errorf("no such block in hash %x: %s", hash, err)
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

// GetOriginBlock returns the genesis block for strict node, but can be any checkpoint for other node
func (c *Chain) GetOriginBlock() *ngtypes.Block {
	return ngtypes.GetGenesisBlock() // TODO: for partial sync func
}
