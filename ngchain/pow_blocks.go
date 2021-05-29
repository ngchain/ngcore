package ngchain

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngblocks"

	"github.com/dgraph-io/badger/v3"

	"github.com/ngchain/ngcore/ngtypes"
)

// GetLatestBlock will return the latest Block in DB.
func (chain *Chain) GetLatestBlock() *ngtypes.Block {
	height := chain.GetLatestBlockHeight()

	block, err := chain.GetBlockByHeight(height)
	if err != nil {
		panic(err)
	}

	return block
}

// GetLatestBlockHash will fetch the latest block from chain and then calc its hash
func (chain *Chain) GetLatestBlockHash() []byte {
	var latestHash []byte

	if err := chain.View(func(txn *badger.Txn) error {
		var err error
		latestHash, err = ngblocks.GetLatestHash(txn)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		log.Error("failed to get latest block hash: %s", err)
		return nil
	}

	return latestHash
}

// GetLatestBlockHeight will fetch the latest block from chain and then return its height
func (chain *Chain) GetLatestBlockHeight() uint64 {
	var latestHeight uint64

	if err := chain.View(func(txn *badger.Txn) error {
		var err error
		latestHeight, err = ngblocks.GetLatestHeight(txn)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		log.Error("failed to get latest block height: %s", err)
		return 0
	}

	return latestHeight
}

// GetLatestCheckpointHash returns the hash of latest checkpoint
func (chain *Chain) GetLatestCheckpointHash() []byte {
	cp := chain.GetLatestCheckpoint()
	return cp.GetHash()
}

// GetLatestCheckpoint returns the latest checkpoint block
func (chain *Chain) GetLatestCheckpoint() *ngtypes.Block {
	b := chain.GetLatestBlock()
	if b.IsGenesis() || b.IsHead() {
		return b
	}

	checkpointHeight := b.Height - b.Height%ngtypes.BlockCheckRound
	b, err := chain.GetBlockByHeight(checkpointHeight)
	if err != nil {
		log.Errorf("error when getting latest checkpoint, maybe chain is broken, please resync: %s", err)
	}

	return b
}

// GetBlockByHeight returns a block by height inputted
func (chain *Chain) GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	if height == 0 {
		return ngtypes.GetGenesisBlock(chain.Network), nil
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

// GetBlockByHash returns a block by hash inputted
func (chain *Chain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	if bytes.Equal(hash, ngtypes.GetGenesisBlockHash(chain.Network)) {
		return ngtypes.GetGenesisBlock(chain.Network), nil
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
func (chain *Chain) GetOriginBlock() *ngtypes.Block {
	var origin *ngtypes.Block
	err := chain.View(func(txn *badger.Txn) error {
		var err error
		origin, err = ngblocks.GetOriginBlock(txn)
		if err != nil {
			return err
		}

		return err
	})
	if err != nil {
		panic(err)
	}

	return origin
}

// ForceApplyBlocks simply checks the block and then calls chain.ForcePutNewBlock
// but **do not** upgrade the state.
// so, after this, dev should do a regenerate or import latest sheet.
func (chain *Chain) ForceApplyBlocks(blocks []*ngtypes.Block) error {
	if err := chain.Update(func(txn *badger.Txn) error {
		for i := 0; i < len(blocks); i++ {
			block := blocks[i]
			//if err := chain.CheckBlock(block); err != nil {
			//	return err
			//}
			// Todo: enhance error check here(based on blocks rather than db)
			if err := block.CheckError(); err != nil {
				return err
			}

			err := chain.ForcePutNewBlock(txn, block)
			if err != nil {
				return fmt.Errorf("failed to force putting new block: %s", err)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
