package ngblocks

import (
	"bytes"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// initWithGenesis will initialize the store with genesis block & vault.
func initWithGenesis() {
	if !hasGenesisBlock() {
		log.Warnf("initializing with genesis block")

		block := ngtypes.GetGenesisBlock()

		if err := store.Update(func(txn *badger.Txn) error {
			hash := block.Hash()
			raw, _ := utils.Proto.Marshal(block)
			log.Debugf("putting block@%d: %x", block.Height, hash)
			err := txn.Set(append(blockPrefix, hash...), raw)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Height)...), hash)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(block.Height))
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, latestHashTag...), hash)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			panic(err)
		}
	}
}

// hasGenesisBlock checks whether the genesis vault is in db.
func hasGenesisBlock() bool {
	var has = false

	if err := store.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(blockPrefix, utils.PackUint64LE(0)...))
		if err != nil {
			return err
		}
		hash, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if hash != nil {
			has = true
		}
		if !bytes.Equal(hash, ngtypes.GetGenesisBlockHash()) {
			panic("wrong genesis block in db")
		}

		return nil
	}); err != nil && err != badger.ErrKeyNotFound {
		panic(err)
	}

	return has
}

// initWithBlockchain initialize the store by importing the external store.
func initWithBlockchain(blocks ...*ngtypes.Block) error {
	/* Put start */
	err := store.Update(func(txn *badger.Txn) error {
		for i := 0; i < len(blocks); i++ {
			block := blocks[i]
			hash := block.Hash()
			raw, _ := utils.Proto.Marshal(block)
			log.Debugf("putting block@%d: %x", block.Height, hash)
			err := txn.Set(append(blockPrefix, hash...), raw)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Height)...), hash)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(block.Height))
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, latestHashTag...), hash)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}
