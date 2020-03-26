package ngchain

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// PutNewVault puts a new vault into db
func (c *Chain) PutNewBlock(block *ngtypes.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	hash, _ := block.CalculateHash()
	if bytes.Compare(hash, ngtypes.GenesisBlockHash) != 0 {
		// when block is not genesis block, checking error
		if block.GetHeight() != 0 {
			if b, _ := c.GetBlockByHeight(block.GetHeight()); b != nil {
				if hashInDB, _ := b.CalculateHash(); bytes.Compare(hash, hashInDB) == 0 {
					return nil
				}
				return fmt.Errorf("has block in same height: %v", b)
			}
		}

		if _, err := c.GetBlockByHash(block.GetPrevHash()); err != nil {
			return fmt.Errorf("no prev block in storage: %x, %v", block.GetPrevHash(), err)
		}
	}

	err := c.db.Update(func(txn *badger.Txn) error {
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
	return err
}

// PutNewVault puts a new vault into db
func (c *Chain) PutNewVault(vault *ngtypes.Vault) error {
	if vault == nil {
		return fmt.Errorf("block is nil")
	}

	hash, _ := vault.CalculateHash()
	if bytes.Compare(hash, ngtypes.GenesisVaultHash) != 0 {
		// when vault is not genesis vault, checking error
		if vault.GetHeight() != 0 {
			if v, _ := c.GetVaultByHeight(vault.GetHeight()); v != nil {
				if hashInDB, _ := v.CalculateHash(); bytes.Compare(hash, hashInDB) == 0 {
					return nil
				}
				return fmt.Errorf("has vault in same height: %v", v)
			}
		}

		if _, err := c.GetVaultByHash(vault.GetPrevHash()); err != nil {
			return fmt.Errorf("no prev vault: %x, %v", vault.GetPrevHash(), err)
		}
	}

	err := c.db.Update(func(txn *badger.Txn) error {
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
	return err
}

// PutNewVault puts a new vault into db
func (c *Chain) PutNewBlockWithVault(vault *ngtypes.Vault, block *ngtypes.Block) error {
	if vault == nil {
		return fmt.Errorf("vault is nil")
	}

	if block == nil {
		return fmt.Errorf("block is nil")
	}

	if vault.GetHeight() != 0 {
		if v, _ := c.GetVaultByHeight(vault.GetHeight()); v != nil {
			return fmt.Errorf("has vault in same height: %v", v)
		}
	}

	if block.GetHeight() != 0 {
		if b, _ := c.GetBlockByHeight(block.GetHeight()); b != nil {
			return fmt.Errorf("has block in same height: %v", b)
		}
	}

	if _, err := c.GetBlockByHash(block.GetPrevHash()); err != nil {
		return err
	}

	if !block.IsHead() {
		return fmt.Errorf("requires a head block to call PutNewBlockWithVault")
	}

	vaultHash, _ := vault.CalculateHash()

	if bytes.Compare(vaultHash, block.Header.PrevVaultHash) != 0 {
		return fmt.Errorf("vault hash is not matching block's prev vault hash")
	}

	err := c.db.Update(func(txn *badger.Txn) error {
		raw, _ := vault.Marshal()
		log.Infof("putting vault@%d: %x", vault.Height, vaultHash)
		err := txn.Set(append(vaultPrefix, vaultHash...), raw)
		if err != nil {
			return err
		}
		err = txn.Set(append(vaultPrefix, utils.PackUint64LE(vault.Height)...), vaultHash)
		if err != nil {
			return err
		}
		err = txn.Set(append(vaultPrefix, LatestHeightTag...), utils.PackUint64LE(vault.Height))
		if err != nil {
			return err
		}
		err = txn.Set(append(vaultPrefix, LatestHashTag...), vaultHash)
		if err != nil {
			return err
		}

		blockHash, _ := block.CalculateHash()
		raw, _ = block.Marshal()
		log.Infof("putting block@%d: %x", block.Header.Height, blockHash)
		err = txn.Set(append(blockPrefix, blockHash...), raw)
		if err != nil {
			return err
		}
		err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Header.Height)...), blockHash)
		if err != nil {
			return err
		}
		err = txn.Set(append(blockPrefix, LatestHeightTag...), utils.PackUint64LE(block.Header.Height))
		if err != nil {
			return err
		}
		err = txn.Set(append(blockPrefix, LatestHashTag...), blockHash)
		if err != nil {
			return err
		}

		return nil
	})
	return err
}

// PutNewChain puts a new chain(vault + block) into db
func (c *Chain) PutNewChain(chain ...Item) error {
	/* Check Start */
	if len(chain) < 3 {
		return fmt.Errorf("chain is nil")
	}

	if err := checkChain(chain...); err != nil {
		return err
	}

	if firstVault, ok := chain[0].(*ngtypes.Vault); !ok {
		return fmt.Errorf("first one of chain shall be an vault")
	} else {
		if hash, _ := firstVault.CalculateHash(); bytes.Compare(hash, ngtypes.GenesisVaultHash) != 0 {
			// not genesis
			_, err := c.GetVaultByHash(firstVault.GetPrevHash())
			if err != nil {
				return fmt.Errorf("the first vault's prevHash is invalid: %s", err)
			}
		}
	}

	if firstBlock, ok := chain[1].(*ngtypes.Block); !ok {
		return fmt.Errorf("second one of chain shall be a block")
	} else {
		if hash, _ := firstBlock.CalculateHash(); bytes.Compare(hash, ngtypes.GenesisBlockHash) != 0 {
			// not genesis
			_, err := c.GetBlockByHash(firstBlock.GetPrevHash())
			if err != nil {
				return fmt.Errorf("the first block@%d's prevHash is invalid: %s", firstBlock.GetHeight(), err)
			}
		}
	}
	/* Check End */

	/* Put start */
	err := c.db.Update(func(txn *badger.Txn) error {
		for i := 0; i < len(chain); i++ {
			switch item := chain[i].(type) {
			case *ngtypes.Block:
				block := item

				if b, _ := c.GetVaultByHeight(block.GetHeight()); b == nil {
					return fmt.Errorf("havent reach the vault height")
				}

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
				if v, _ := c.GetVaultByHeight(vault.GetHeight()); v == nil {
					return fmt.Errorf("havent reach the vault height")
				}
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