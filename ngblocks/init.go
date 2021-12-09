package ngblocks

import (
	"bytes"
	"encoding/binary"

	"github.com/c0mm4nd/dbolt"
	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

var ErrDBNotInit = errors.New("DB not initialized")
var ErrMalformedGenesisBlock = errors.New("malformed genesis block")

// initWithGenesis will initialize the store with genesis block & vault.
func (store *BlockStore) initWithGenesis() {
	if !store.hasGenesisBlock(store.Network) {
		log.Warnf("initializing with genesis block")

		block := ngtypes.GetGenesisBlock(store.Network)

		if err := store.Update(func(txn *dbolt.Tx) error {
			blockBucket := txn.Bucket(storage.BlockBucketName)

			hash := block.GetHash()
			raw, _ := rlp.EncodeToBytes(block)

			log.Infof("putting block@%d: %x", block.Header.Height, hash)

			err := blockBucket.Put(hash, raw)
			if err != nil {
				return err
			}
			err = blockBucket.Put(utils.PackUint64LE(block.Header.Height), hash)
			if err != nil {
				return err
			}

			err = blockBucket.Put(storage.LatestHeightTag, utils.PackUint64LE(block.Header.Height))
			if err != nil {
				return err
			}
			err = blockBucket.Put(storage.LatestHashTag, hash)
			if err != nil {
				return err
			}

			err = blockBucket.Put(storage.OriginHeightTag, utils.PackUint64LE(block.Header.Height))
			if err != nil {
				return err
			}
			err = blockBucket.Put(storage.OriginHashTag, hash)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			panic(err)
		}
	}
}

// hasGenesisBlock checks whether the genesis block is in db.
func (store *BlockStore) hasGenesisBlock(network ngtypes.Network) bool {
	if err := store.View(func(txn *dbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		if blockBucket == nil {
			return ErrDBNotInit
		}

		hash := blockBucket.Get(utils.PackUint64LE(0))
		if hash == nil {
			return storage.ErrKeyNotFound
		}
		if !bytes.Equal(hash, ngtypes.GetGenesisBlock(network).GetHash()) {
			panic(errors.Wrapf(ErrMalformedGenesisBlock, "genesis block hash mismatch %x and %x", hash, ngtypes.GetGenesisBlock(network).GetHash()))
		}

		return nil
	}); err != nil {
		return false
	}

	return true
}

// hasOrigin checks whether the genesis origin is in db.
func (store *BlockStore) hasOrigin(network ngtypes.Network) bool {
	if err := store.View(func(txn *dbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		if blockBucket == nil {
			return ErrDBNotInit
		}

		height := blockBucket.Get(storage.OriginHeightTag)
		if height == nil {
			return storage.ErrKeyNotFound
		}

		hash := blockBucket.Get(storage.OriginHashTag)
		if hash == nil {
			return storage.ErrKeyNotFound
		}

		rawBlock := blockBucket.Get(hash)
		if rawBlock == nil {
			return storage.ErrKeyNotFound
		}

		var originBlock ngtypes.Block
		err := rlp.DecodeBytes(rawBlock, &originBlock)
		if err != nil {
			return err
		}

		if originBlock.Header.Network != network || originBlock.Header.Height != binary.LittleEndian.Uint64(height) {
			panic(ErrMalformedGenesisBlock)
		}

		return nil
	}); err != nil {
		return false
	}

	return true
}

// initWithBlockchain initialize the store by importing the external store.
// func (store *BlockStore) initWithBlockchain(blocks ...*ngtypes.Block) error {
//	/* Put start */
//	err := store.Update(func(txn *dbolt.Tx) error {
//		for i := 0; i < len(blocks); i++ {
//			block := blocks[i]
//			hash := block.Hash()
//			raw, _ := rlp.EncodeToBytes(block)
//			log.Infof("putting block@%d: %x", block.Height, hash)
//			err := txn.Set(append(blockPrefix, hash...), raw)
//			if err != nil {
//				return err
//			}
//			err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Height)...), hash)
//			if err != nil {
//				return err
//			}
//			err = txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(block.Height))
//			if err != nil {
//				return err
//			}
//			err = txn.Set(append(blockPrefix, latestHashTag...), hash)
//			if err != nil {
//				return err
//			}
//		}
//		return nil
//	})
//
//	return err
// }

func (store *BlockStore) InitFromCheckpoint(block *ngtypes.Block) error {
	err := store.Update(func(txn *dbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		hash := block.GetHash()
		raw, _ := rlp.EncodeToBytes(block)
		log.Infof("putting block@%d: %x", block.Header.Height, hash)
		err := blockBucket.Put(hash, raw)
		if err != nil {
			return err
		}
		err = blockBucket.Put(utils.PackUint64LE(block.Header.Height), hash)
		if err != nil {
			return err
		}

		err = blockBucket.Put(storage.LatestHeightTag, utils.PackUint64LE(block.Header.Height))
		if err != nil {
			return err
		}
		err = blockBucket.Put(storage.LatestHashTag, hash)
		if err != nil {
			return err
		}

		err = blockBucket.Put(storage.OriginHeightTag, utils.PackUint64LE(block.Header.Height))
		if err != nil {
			return err
		}
		err = blockBucket.Put(storage.OriginHashTag, hash)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}
