package consensus

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngtypes"
)

// checkChain is a helper to check whether the items are aligned as a chain.
func (c *Consensus) checkChain(items ...ngchain.Item) error {
	if len(items) == 0 {
		return fmt.Errorf("empty chain")
	}

	if len(items) == 1 {
		return c.checkVault(items[0].(*ngtypes.Vault))
	}

	var curBlock, prevBlock *ngtypes.Block
	var curVault, prevVault *ngtypes.Vault

	var prevBlockHash, prevVaultHash []byte

	var err error
	firstVault := items[0].(*ngtypes.Vault)
	firstBlock := items[1].(*ngtypes.Block)
	if firstBlock == ngtypes.GetGenesisBlock() && firstVault == ngtypes.GetGenesisVault() {
		prevBlock = firstBlock
		prevVault = firstVault
		prevBlockHash = ngtypes.GenesisBlockHash
		prevVaultHash = ngtypes.GenesisVaultHash
	} else {
		prevBlock, err = c.GetBlockByHash(firstBlock.GetPrevHash())
		if err != nil {
			return err
		}
		prevBlockHash, _ = prevBlock.CalculateHash()

		prevVault, err = c.GetVaultByHash(prevBlock.Header.PrevVaultHash)
		if err != nil {
			return err
		}
		prevVaultHash, _ = prevVault.CalculateHash()
	}

	for i := 0; i < len(items); i++ {
		switch items[i].(type) {
		case *ngtypes.Vault:
			curVault = items[i].(*ngtypes.Vault)
			if err = curVault.CheckError(); err != nil {
				return err
			}

			// prevVaultHash, _ := prevVault.CalculateHash()
			if curVault != nil {
				hash, _ := curVault.CalculateHash()
				if !bytes.Equal(prevVaultHash, curVault.GetPrevHash()) {
					return fmt.Errorf("vault@%d:%x 's prevHash: %x is not matching vault@%d:%x 's hash", curVault.GetHeight(), hash, curVault.GetPrevHash(), prevVault.GetHeight(), prevVaultHash)
				}

				prevVault = curVault
				prevVaultHash = hash
			}

		case *ngtypes.Block:
			curBlock = items[i].(*ngtypes.Block)
			if err = curBlock.CheckError(); err != nil {
				return err
			}

			if err = c.checkBlockTarget(curBlock, prevBlock, curVault); err != nil {
				return err
			}

			if err = c.TxPool.CheckTxs(curBlock.Txs...); err != nil {
				return err
			}

			if curBlock != nil {
				hash, _ := curBlock.CalculateHash()
				// prevBlockHash, _ := prevBlock.CalculateHash()
				if !bytes.Equal(prevBlockHash, curBlock.GetPrevHash()) {
					return fmt.Errorf("block@%d:%x 's prevBlockHash: %x is not matching block@%d:%x 's hash", curBlock.GetHeight(), hash, curBlock.GetPrevHash(), prevBlock.GetHeight(), prevBlockHash)
				}

				prevBlock = curBlock
				prevBlockHash = hash
			}

		default:
			return fmt.Errorf("unknown type: %curVault", items[i])
		}

	}

	return nil
}
