package chain

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

	for i := 1; i < len(items); i++ {
		switch item := items[i].(type) {
		case *ngtypes.Block:
			if curBlock == nil {
				curBlock = item
			}

			if prevBlock != nil {
				prevHash, _ := prevBlock.CalculateHash()
				if bytes.Compare(prevHash, curBlock.GetPrevHash()) != 0 {
					curHash, _ := curBlock.CalculateHash()
					return fmt.Errorf("block@%d:%x 's prevHash: %x is not matching block@%d:%x 's hash", prevBlock.GetHeight(), prevHash, curBlock.GetHeight(), curBlock.GetPrevHash(), curHash)
				}
			}

			prevBlock = curBlock

		case *ngtypes.Vault:
			if curVault == nil {
				curVault = item
			}

			if prevVault != nil {
				prevHash, _ := prevVault.CalculateHash()
				if bytes.Compare(prevHash, curVault.GetPrevHash()) != 0 {
					curHash, _ := curVault.CalculateHash()
					return fmt.Errorf("vault@%d:%x 's prevHash: %x is not matching vault@%d:%x 's hash", prevVault.GetHeight(), prevHash, curVault.GetHeight(), curVault.GetPrevHash(), curHash)
				}
			}

			prevVault = curVault
		}
		hash, _ := items[i-1].CalculateHash()
		if bytes.Compare(hash, items[i].GetPrevHash()) != 0 {
			return fmt.Errorf("items are not a valid chain, item")
		}
	}

	return nil
}
