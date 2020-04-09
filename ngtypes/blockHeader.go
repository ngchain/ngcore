package ngtypes

import (
	"encoding/binary"

	"github.com/ngchain/cryptonight-go"
)

/* Header Start */

// IsHead will check whether the Block is the checkpoint
func (m *BlockHeader) IsHead() bool {
	return m.GetHeight()%BlockCheckRound == 0
}

func (m *BlockHeader) IsTail() bool {
	return m.GetHeight()%BlockCheckRound+1 == BlockCheckRound
}

func (m *BlockHeader) IsGenesisBlock() bool {
	return m.GetHeight() == 0
}

// GetPoWBlob will return a complete blob for block hash
func (m *BlockHeader) GetPoWBlob(nonce []byte) []byte {
	raw := make([]byte, 148)

	binary.LittleEndian.PutUint32(raw[0:4], uint32(m.GetVersion()))
	copy(raw[4:36], m.GetPrevBlockHash())
	copy(raw[36:68], m.GetPrevVaultHash())
	copy(raw[68:100], m.GetTrieHash())
	binary.LittleEndian.PutUint64(raw[100:108], uint64(m.GetTimestamp()))
	copy(raw[108:140], m.GetTarget()) // uint256

	if nonce == nil {
		copy(raw[140:148], m.GetNonce()) // 8
	} else {
		copy(raw[140:148], nonce) // 8
	}

	return raw
}

// CalculateHash will help you get the hash of block
func (m *BlockHeader) CalculateHash() []byte {
	blob := m.GetPoWBlob(nil)
	return cryptonight.Sum(blob, 0)
}

func (m *BlockHeader) IsUnsealing() bool {
	return m.GetTrieHash() != nil
}

func (m *BlockHeader) IsSealed() bool {
	return m.GetNonce() != nil
}

/* Header End */
