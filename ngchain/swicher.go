package ngchain

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// SwitchTo changes the items in db, requiring the first one of chain is an vault. The chain should follow the order vault0-block0-block1...-block6-vault2-block7...
func (c *Chain) SwitchTo(chain ...Item) error {
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
		for i := range chain {
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
				return nil
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
				return nil
			default:
				panic("unknown item")
			}

		}
		return nil
	})
	return err
}
