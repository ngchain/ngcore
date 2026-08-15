package ngtypes

import (
	"bytes"
	"errors"

	"github.com/c0mm4nd/rlp"
	"github.com/cbergoon/merkletree"

	"github.com/ngchain/ngcore/utils"
)

// BlockHeader is the fix-sized header of the block, which is able to
// describe itself.
type BlockHeader struct {
	Network Network // 1

	Height    uint64 // 4
	Timestamp uint64 // 4

	PrevBlockHash []byte // 32
	TxTrieHash    []byte // 32
	WitnessRoot   []byte // 32

	Difficulty []byte // 32
	Nonce      []byte `rlp:"tail"` // 8
}

// GetHeight returns the height of the block.
func (x *BlockHeader) GetHeight() uint64 {
	return x.Height
}

// GetPrevHash returns the hash of the previous block.
func (x *BlockHeader) GetPrevHash() []byte {
	return x.PrevBlockHash
}

// GetTimestamp returns the timestamp of the block.
func (x *BlockHeader) GetTimestamp() uint64 {
	return x.Timestamp
}

// CalculateHash calcs the hash of the Block header with sha3_256, aiming to
// get the merkletree hash when summarizing subs into the header.
func (x *BlockHeader) CalculateHash() ([]byte, error) {
	raw, err := rlp.EncodeToBytes(x)
	if err != nil {
		return nil, err
	}

	return utils.KeccakSum256(raw), nil
}

func (x *BlockHeader) GetHash() []byte {
	hash, err := x.CalculateHash()
	if err != nil {
		panic(err)
	}
	return hash
}

// ErrNotBlockHeader means the var is not a block header
var ErrNotBlockHeader = errors.New("not a block header")

// Equals checks whether the block headers equal
func (x *BlockHeader) Equals(other merkletree.Content) (bool, error) {
	header, ok := other.(*BlockHeader)
	if !ok {
		return false, ErrNotBlockHeader
	}

	if x.Network != header.Network {
		return false, nil
	}
	if x.Height != header.Height {
		return false, nil
	}
	if x.Timestamp != header.Timestamp {
		return false, nil
	}
	if !bytes.Equal(x.PrevBlockHash, header.PrevBlockHash) {
		return false, nil
	}
	if !bytes.Equal(x.TxTrieHash, header.TxTrieHash) {
		return false, nil
	}
	if !bytes.Equal(x.WitnessRoot, header.WitnessRoot) {
		return false, nil
	}
	if !bytes.Equal(x.Difficulty, header.Difficulty) {
		return false, nil
	}
	if !bytes.Equal(x.Nonce, header.Nonce) {
		return false, nil
	}

	return true, nil
}
