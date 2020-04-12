package ngtypes

import (
	"encoding/binary"

	"github.com/ngchain/cryptonight-go"
)

// IsHead will check whether the Block is the checkpoint
func (m *BlockHeader) IsHead() bool {
	return m.GetHeight()%BlockCheckRound == 0
}

func (m *BlockHeader) IsTail() bool {
	return (m.GetHeight()+1)%BlockCheckRound == 0
}

// GetPoWBlob will return a complete blob for block hash
func (m *BlockHeader) GetPoWBlob(nonce []byte) []byte {
	raw := make([]byte, 144)

	copy(raw[0:32], m.GetPrevBlockHash())
	copy(raw[32:64], m.GetSheetHash())
	copy(raw[64:96], m.GetTrieHash())
	binary.LittleEndian.PutUint64(raw[96:104], uint64(m.GetTimestamp()))
	copy(raw[104:136], m.GetTarget()) // uint256

	if nonce == nil {
		copy(raw[136:144], m.GetNonce()) // 8
	} else {
		copy(raw[136:144], nonce) // 8
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
