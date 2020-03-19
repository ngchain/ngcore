package chain

import (
	"bytes"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/utils"
)

func (c *Chain) InitWithGenesis() {
	if !c.HasGenesisBlock() {
		log.Infof("initializing with genesis block")
		err := c.PutNewBlock(ngtypes.GetGenesisBlock())
		if err != nil {
			panic(err)
		}
	}

	if !c.HasGenesisVault() {
		log.Infof("initializing with genesis vault")
		err := c.PutNewVault(ngtypes.GetGenesisVault())
		if err != nil {
			panic(err)
		}
	}
}

func (c *Chain) HasGenesisBlock() bool {
	var has = false
	err := c.db.View(func(txn *badger.Txn) error {
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
		if bytes.Compare(hash, ngtypes.GenesisBlockHash) != 0 {
			panic("wrong genesis block in db")
		}

		return nil
	})
	if err != nil && err != badger.ErrKeyNotFound {
		panic(err)
	}

	return has
}

func (c *Chain) HasGenesisVault() bool {
	var has = false
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(vaultPrefix, utils.PackUint64LE(0)...))
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
		if bytes.Compare(hash, ngtypes.GenesisVaultHash) != 0 {
			panic("wrong genesis vault in db")
		}

		return nil
	})
	if err != nil && err != badger.ErrKeyNotFound {
		panic(err)
	}

	return has
}
