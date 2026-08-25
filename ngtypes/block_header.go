package ngtypes

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"time"

	"github.com/c0mm4nd/rlp"
	"github.com/cbergoon/merkletree"
	"github.com/ngchain/astrobwt"

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
	// Coinbase is the miner's address. Uncles carry only their header, so
	// the miner address must live in the header (and the pow preimage) for
	// a nephew to pay the orphaned miner without carrying the uncle's body.
	Coinbase []byte // 32
	// UnclesHash commits to the block's uncle headers (GHOST). It is
	// folded into the pow preimage so uncles cannot be swapped after
	// sealing. All-zero when the block carries no uncles.
	UnclesHash []byte // 32
	Nonce      []byte `rlp:"tail"` // 8
}

// GetPoWRawHeader builds the fixed-size pow preimage from the header
// fields. When nonce is non-nil it overrides the header's own Nonce.
func (x *BlockHeader) GetPoWRawHeader(nonce []byte) []byte {
	// Network(1) + Height(8) + Timestamp(8) + PrevBlockHash(32) +
	// TxTrieHash(32) + WitnessRoot(32) + Difficulty(32) + Coinbase(32) +
	// UnclesHash(32) + Nonce(8) = 217
	raw := make([]byte, 217)

	raw[0] = byte(x.Network)
	binary.LittleEndian.PutUint64(raw[1:], x.Height)
	binary.LittleEndian.PutUint64(raw[9:17], x.Timestamp)
	copy(raw[17:49], x.PrevBlockHash)
	copy(raw[49:81], x.TxTrieHash)
	copy(raw[81:113], x.WitnessRoot)
	copy(raw[113:145], utils.ReverseBytes(x.Difficulty)) // uint256
	copy(raw[145:177], x.Coinbase)
	copy(raw[177:209], x.UnclesHash)

	if nonce == nil {
		copy(raw[209:217], x.Nonce)
	} else {
		copy(raw[209:217], nonce)
	}

	return raw
}

// PowHash returns the astrobwt pow hash of the header. Safe to call on a
// bare uncle header to verify its standalone proof of work.
func (x *BlockHeader) PowHash() []byte {
	hash := astrobwt.POW_0alloc(x.GetPoWRawHeader(nil))
	return hash[:]
}

// ActualDiff returns the difficulty the header's nonce actually achieved.
func (x *BlockHeader) ActualDiff() *big.Int {
	return new(big.Int).Div(MaxTarget, new(big.Int).SetBytes(x.PowHash()))
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

	return utils.Hash256(raw), nil
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

// checkStandaloneError validates an uncle header on its own: well-formed
// fixed-size fields, not the genesis, and a nonce that meets the header's
// OWN declared difficulty (real proof of work). Whether that declared
// difficulty is correct for the uncle's slot is a chain-context check.
func (x *BlockHeader) checkStandaloneError() error {
	if len(x.PrevBlockHash) != HashSize || len(x.TxTrieHash) != HashSize ||
		len(x.WitnessRoot) != HashSize || len(x.UnclesHash) != HashSize ||
		len(x.Coinbase) != AddressSize {
		return errors.New("uncle header has a malformed fixed-size field")
	}
	if len(x.Difficulty) == 0 || len(x.Difficulty) > DiffSize {
		return errors.New("uncle header has a malformed difficulty")
	}
	if len(x.Nonce) != NonceSize {
		return errors.New("uncle header nonce length is incorrect")
	}
	if x.Height == 0 {
		return errors.New("the genesis block cannot be an uncle")
	}
	if x.Timestamp > uint64(time.Now().UnixMilli())+TimestampDriftTolerance {
		return errors.New("uncle header timestamp is too far in the future")
	}

	diff := new(big.Int).SetBytes(x.Difficulty)
	target := new(big.Int).Div(MaxTarget, diff)
	if new(big.Int).SetBytes(x.PowHash()).Cmp(target) >= 0 {
		return errors.New("uncle header nonce does not meet its declared difficulty")
	}

	return nil
}

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
	if !bytes.Equal(x.Coinbase, header.Coinbase) {
		return false, nil
	}
	if !bytes.Equal(x.UnclesHash, header.UnclesHash) {
		return false, nil
	}
	if !bytes.Equal(x.Nonce, header.Nonce) {
		return false, nil
	}

	return true, nil
}
