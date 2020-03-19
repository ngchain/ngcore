package chain

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/utils"
)

// SwitchTo changes the items in db, requires the first one is vault
func (c *Chain) SwitchTo(chain ...Item) error {
	if len(chain) == 0 {
		return fmt.Errorf("chain is nil")
	}

	if err := checkChain(chain...); err != nil {
		return err
	}

	if _, ok := chain[0].(*ngtypes.Vault); !ok {
		return fmt.Errorf("first one of chain shall be the vault")
	}

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
