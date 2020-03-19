package chain

import (
	"bytes"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/ngin-network/ngcore/ngtypes"
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

func checkChain(chain ...Item) error {
	var curBlock, prevBlock *ngtypes.Block
	var curVault, prevVault *ngtypes.Vault

	for i := 0; i < len(chain); i++ {
		switch item := chain[i-1].(type) {
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
		hash, _ := chain[i-1].CalculateHash()
		if bytes.Compare(hash, chain[i].GetPrevHash()) != 0 {
			return fmt.Errorf("chain is not a valid chain, item")
		}
	}

	return nil
}
