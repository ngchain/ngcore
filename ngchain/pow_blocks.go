package ngchain

import (
	"bytes"
	"fmt"
	"github.com/ngchain/ngcore/ngblocks"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

// GetLatestBlock will return the latest Block in DB.
func GetLatestBlock() *ngtypes.Block {
	height := GetLatestBlockHeight()

	block, err := GetBlockByHeight(height)
	if err != nil {
		log.Error(err)
	}

	return block
}

// GetLatestBlockHash will fetch the latest block from chain and then calc its hash
func GetLatestBlockHash() []byte {
	return GetLatestBlock().Hash()
}

// GetLatestBlockHeight will fetch the latest block from chain and then return its height
func GetLatestBlockHeight() uint64 {
	var latestHeight uint64

	if err := chain.View(func(txn *badger.Txn) error {
		var err error
		latestHeight, err = ngblocks.GetLatestHeight(txn)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		log.Error(err)
		return 0
	}

	return latestHeight
}

// GetLatestCheckpointHash returns the hash of latest checkpoint
func GetLatestCheckpointHash() []byte {
	cp := GetLatestCheckpoint()
	return cp.Hash()
}

// GetLatestCheckpoint returns the latest checkpoint block
func GetLatestCheckpoint() *ngtypes.Block {
	b := GetLatestBlock()
	if b.IsGenesis() {
		return b
	}

	if err := chain.View(func(txn *badger.Txn) error {
		for {
			hash := b.GetPrevHash()
			b, err := ngblocks.GetBlockByHash(txn, hash)
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
func GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	if height == 0 {
		return ngtypes.GetGenesisBlock(), nil
	}

	var block = &ngtypes.Block{}

	if err := chain.View(func(txn *badger.Txn) error {
		var err error
		block, err = ngblocks.GetBlockByHeight(txn, height)
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
func GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	if bytes.Equal(hash, ngtypes.GetGenesisBlockHash()) {
		return ngtypes.GetGenesisBlock(), nil
	}

	if len(hash) != 32 {
		return nil, fmt.Errorf("%x is not a legal hash", hash)
	}

	var block = &ngtypes.Block{}

	if err := chain.View(func(txn *badger.Txn) error {
		var err error
		block, err = ngblocks.GetBlockByHash(txn, hash)
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
func GetOriginBlock() *ngtypes.Block {
	return ngtypes.GetGenesisBlock() // TODO: for partial sync func
}

// ForceApplyBlocks checks the block and then calls ngchain's PutNewBlock, after which update the state
func ForceApplyBlocks(blocks []*ngtypes.Block) error {
	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		if err := CheckBlock(block); err != nil {
			return err
		}

		err := ngblocks.ForcePutNewBlock(block)
		if err != nil {
			return err
		}
	}

	return nil
}
