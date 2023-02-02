package blockchain

import (
	"bytes"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// GetLatestBlock will return the latest Block in DB.
func (chain *Chain) GetLatestBlock() ngtypes.Block {
	return chain.getLatestBlock()
}

func (chain *Chain) getLatestBlock() *ngtypes.FullBlock {
	height := chain.GetLatestBlockHeight()

	block, err := chain.getBlockByHeight(height)
	if err != nil {
		panic(err)
	}

	return block
}

// GetLatestBlockHash will fetch the latest block from chain and then calc its hash.
func (chain *Chain) GetLatestBlockHash() []byte {
	var latestHash []byte

	if err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		var err error
		latestHash, err = ngblocks.GetLatestHash(blockBucket)
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

// GetLatestBlockHeight will fetch the latest block from chain and then return its height.
func (chain *Chain) GetLatestBlockHeight() uint64 {
	var latestHeight uint64

	if err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		var err error
		latestHeight, err = ngblocks.GetLatestHeight(blockBucket)
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

// GetLatestCheckpointHash returns the hash of the latest checkpoint.
func (chain *Chain) GetLatestCheckpointHash() []byte {
	cp := chain.GetLatestCheckpoint()
	return cp.GetHash()
}

// GetLatestCheckpoint returns the latest checkpoint block.
func (chain *Chain) GetLatestCheckpoint() *ngtypes.FullBlock {
	b := chain.getLatestBlock()
	if b.IsGenesis() || b.IsHead() {
		return b
	}

	checkpointHeight := b.GetHeight() - b.GetHeight()%ngtypes.BlockCheckRound
	b, err := chain.getBlockByHeight(checkpointHeight)
	if err != nil {
		log.Errorf("error when getting latest checkpoint, maybe chain is broken, please resync: %s", err)
	}

	return b
}

// GetBlockByHeight returns a block by height inputted.
func (chain *Chain) GetBlockByHeight(height uint64) (ngtypes.Block, error) {
	return chain.getBlockByHeight(height)
}

func (chain *Chain) getBlockByHeight(height uint64) (*ngtypes.FullBlock, error) {
	if height == 0 {
		return ngtypes.GetGenesisBlock(chain.Network), nil
	}

	block := &ngtypes.FullBlock{}

	if err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		var err error
		block, err = ngblocks.GetBlockByHeight(blockBucket, height)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return block, nil
}

// GetBlockByHash returns a block by hash inputted.
func (chain *Chain) GetBlockByHash(hash []byte) (ngtypes.Block, error) {
	return chain.getBlockByHash(hash)
}

func (chain *Chain) getBlockByHash(hash []byte) (*ngtypes.FullBlock, error) {
	if bytes.Equal(hash, ngtypes.GetGenesisBlock(chain.Network).GetHash()) {
		return ngtypes.GetGenesisBlock(chain.Network), nil
	}

	if len(hash) != 32 {
		return nil, errors.Wrapf(ngtypes.ErrHashSize, "%x is not a legal hash", hash)
	}

	block := &ngtypes.FullBlock{}

	if err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		var err error
		block, err = ngblocks.GetBlockByHash(blockBucket, hash)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return block, nil
}

// GetOriginBlock returns the genesis block for strict node, but can be any checkpoint for other node.
func (chain *Chain) GetOriginBlock() *ngtypes.FullBlock {
	var origin *ngtypes.FullBlock
	err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		var err error
		origin, err = ngblocks.GetOriginBlock(blockBucket)
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
// so, after this, dev should do a regeneration or import the latest sheet.
func (chain *Chain) ForceApplyBlocks(blocks []*ngtypes.FullBlock) error {
	if err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		txBucket := txn.Bucket(storage.TxBucketName)

		for i := 0; i < len(blocks); i++ {
			block := blocks[i]
			// if err := chain.CheckBlock(block); err != nil {
			//	return err
			// }
			// Todo: enhance error check here(based on blocks rather than db)
			if err := block.CheckError(); err != nil {
				return err
			}

			err := chain.ForcePutNewBlock(blockBucket, txBucket, block)
			if err != nil {
				return errors.Wrap(err, "failed to force putting new block")
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
