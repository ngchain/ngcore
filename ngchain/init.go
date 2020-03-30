package ngchain

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (c *Chain) InitWithGenesis() {
	if !c.HasGenesisBlock() {
		log.Infof("initializing with genesis block")
		block := ngtypes.GetGenesisBlock()
		err := c.db.Update(func(txn *badger.Txn) error {
			hash, _ := block.CalculateHash()
			raw, _ := block.Marshal()
			log.Infof("putting block@%d: %x", block.Header.Height, hash)
			err := txn.Set(append(blockPrefix, hash...), raw)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Header.Height)...), hash)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, LatestHeightTag...), utils.PackUint64LE(block.Header.Height))
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, LatestHashTag...), hash)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	}

	if !c.HasGenesisVault() {
		log.Infof("initializing with genesis vault")
		vault := ngtypes.GetGenesisVault()
		err := c.db.Update(func(txn *badger.Txn) error {
			hash, _ := vault.CalculateHash()
			raw, _ := vault.Marshal()
			log.Infof("putting vault@%d: %x", vault.Height, hash)
			err := txn.Set(append(vaultPrefix, hash...), raw)
			if err != nil {
				return err
			}
			err = txn.Set(append(vaultPrefix, utils.PackUint64LE(vault.Height)...), hash)
			if err != nil {
				return err
			}
			err = txn.Set(append(vaultPrefix, LatestHeightTag...), utils.PackUint64LE(vault.Height))
			if err != nil {
				return err
			}
			err = txn.Set(append(vaultPrefix, LatestHashTag...), hash)
			if err != nil {
				return err
			}
			return nil
		})
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
		if !bytes.Equal(hash, ngtypes.GenesisBlockHash) {
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
		if !bytes.Equal(hash, ngtypes.GenesisVaultHash) {
			panic("wrong genesis vault in db")
		}

		return nil
	})
	if err != nil && err != badger.ErrKeyNotFound {
		panic(err)
	}

	return has
}

func (c *Chain) InitWithChain(chain ...Item) error {

	/* Check Start */
	if len(chain) < 3 {
		return fmt.Errorf("chain is nil")
	}

	if err := checkChain(chain...); err != nil {
		return err
	}
	/* Check End */

	/* Put start */
	err := c.db.Update(func(txn *badger.Txn) error {
		for i := 0; i < len(chain); i++ {
			switch item := chain[i].(type) {
			case *ngtypes.Block:
				block := item
				hash, _ := block.CalculateHash()
				raw, _ := block.Marshal()
				log.Infof("putting block@%d: %x", block.Header.Height, hash)
				err := txn.Set(append(blockPrefix, hash...), raw)
				if err != nil {
					return err
				}
				err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Header.Height)...), hash)
				if err != nil {
					return err
				}
				err = txn.Set(append(blockPrefix, LatestHeightTag...), utils.PackUint64LE(block.Header.Height))
				if err != nil {
					return err
				}
				err = txn.Set(append(blockPrefix, LatestHashTag...), hash)
				if err != nil {
					return err
				}
			case *ngtypes.Vault:
				vault := item
				hash, _ := vault.CalculateHash()
				raw, _ := vault.Marshal()
				log.Infof("putting vault@%d: %x", vault.Height, hash)
				err := txn.Set(append(vaultPrefix, hash...), raw)
				if err != nil {
					return err
				}
				err = txn.Set(append(vaultPrefix, utils.PackUint64LE(vault.Height)...), hash)
				if err != nil {
					return err
				}
				err = txn.Set(append(vaultPrefix, LatestHeightTag...), utils.PackUint64LE(vault.Height))
				if err != nil {
					return err
				}
				err = txn.Set(append(vaultPrefix, LatestHashTag...), hash)
				if err != nil {
					return err
				}
			default:
				panic("unknown item")
			}
		}
		return nil
	})
	return err
}
