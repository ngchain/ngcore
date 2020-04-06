package ngchain

import (
	"bytes"
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
)

const LatestHeightTag = "height"
const LatestHashTag = "hash"

// Item is an interface to block-like structures
type Item interface {
	proto.Message
	CalculateHash() ([]byte, error)
	GetHeight() uint64
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	GetPrevHash() []byte
}

// checkChain is a helper to check whether the items are aligned as a chain.
func checkChain(items ...Item) error {
	var curBlock, prevBlock *ngtypes.Block
	var curVault, prevVault *ngtypes.Vault

	for i := 0; i < len(items); i++ {
		switch item := items[i].(type) {
		case *ngtypes.Block:
			curBlock = item

			if prevBlock != nil {
				prevBlockHash, _ := prevBlock.CalculateHash()
				if !bytes.Equal(prevBlockHash, curBlock.GetPrevHash()) {
					curHash, _ := curBlock.CalculateHash()
					return fmt.Errorf("block@%d:%x 's prevBlockHash: %x is not matching block@%d:%x 's hash", curBlock.GetHeight(), curHash, curBlock.GetPrevHash(), prevBlock.GetHeight(), prevBlockHash)
				}
			}

			prevBlock = curBlock

		case *ngtypes.Vault:
			curVault = item

			if prevVault != nil {
				prevVaultHash, _ := prevVault.CalculateHash()
				if bytes.Equal(prevVaultHash, curVault.GetPrevHash()) {
					curVaultHash, _ := curVault.CalculateHash()
					return fmt.Errorf("vault@%d:%x 's prevHash: %x is not matching vault@%d:%x 's hash", curVault.GetHeight(), curVaultHash, curVault.GetPrevHash(), prevVault.GetHeight(), prevVaultHash)
				}
			}

			prevVault = curVault
		}
	}

	return nil
}
