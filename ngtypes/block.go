package ngtypes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoimpl"
	"math/big"
	"time"

	"golang.org/x/crypto/sha3"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/cryptonight-go"

	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("types")

// IsUnsealing checks whether the block is unsealing.
func (x *Block) IsUnsealing() bool {
	return x.GetTrieHash() != nil
}

// IsSealed checks whether the block is sealed.
func (x *Block) IsSealed() bool {
	return x.GetNonce() != nil
}

// IsHead will check whether the Block is the head(checkpoint).
func (x *Block) IsHead() bool {
	return x.GetHeight()%BlockCheckRound == 0
}

// IsTail will check whether the Block is the tail(the one before head).
func (x *Block) IsTail() bool {
	return (x.GetHeight()+1)%BlockCheckRound == 0
}

// IsGenesis will check whether the Block is the genesis block.
func (x *Block) IsGenesis() bool {
	return bytes.Equal(x.Hash(), GenesisBlockHash)
}

// GetPoWBlob will return a complete blob for block hash.
func (x *Block) GetPoWBlob(nonce []byte) []byte {
	lenRaw := HashSize + HashSize +
		TimestampSize + // timestamp
		HashSize + // unknown length
		HashSize +
		NonceSize
	raw := make([]byte, lenRaw)

	l := 0

	copy(raw[l:l+HashSize], x.GetPrevBlockHash())
	l += HashSize

	copy(raw[l:l+HashSize], x.GetTrieHash())
	l += HashSize

	binary.LittleEndian.PutUint64(raw[l:l+TimestampSize], uint64(x.GetTimestamp()))
	l += TimestampSize

	copy(raw[l:l+HashSize], x.GetDifficulty()) // uint256
	l += HashSize

	if nonce == nil {
		copy(raw[l:l+NonceSize], x.GetNonce())
	} else {
		copy(raw[l:l+NonceSize], nonce)
	}

	return raw
}

// PowHash will help you get the pow hash of block.
func (x *Block) PowHash() []byte {
	blob := x.GetPoWBlob(nil)
	return cryptonight.Sum(blob, 0)
}

// ToUnsealing converts a bare block to an unsealing block.
func (x *Block) ToUnsealing(txsWithGen []*Tx) (*Block, error) {
	if txsWithGen[0].GetType() != TxType_GENERATE {
		return nil, fmt.Errorf("first tx shall be a generate")
	}

	for i := 1; i < len(txsWithGen); i++ {
		if txsWithGen[i].GetType() == TxType_GENERATE {
			return nil, fmt.Errorf("except first, other tx shall not be a generate")
		}
	}

	x.TrieHash = NewTxTrie(txsWithGen).TrieRoot()
	x.Txs = txsWithGen

	return x, nil
}

// ToSealed converts an unsealing block to a sealed block.
func (x *Block) ToSealed(nonce []byte) (*Block, error) {
	if !x.IsUnsealing() {
		return nil, fmt.Errorf("the block is bare")
	}

	x.Nonce = nonce

	return x, nil
}

// verifyNonce will verify whether the nonce meets the target.
func (x *Block) verifyNonce() error {
	diff := new(big.Int).SetBytes(x.GetDifficulty())
	target := new(big.Int).Div(MaxTarget, diff)

	if new(big.Int).SetBytes(x.PowHash()).Cmp(target) < 0 {
		return nil
	}

	return fmt.Errorf("block@%d's nonce %x is invalid", x.GetHeight(), x.GetNonce())
}

// GetActualDiff returns the diff decided by nonce.
func (x *Block) GetActualDiff() *big.Int {
	return new(big.Int).Div(MaxTarget, new(big.Int).SetBytes(x.PowHash()))
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N.
func NewBareBlock(height uint64, prevSheetHash, prevBlockHash []byte, diff *big.Int) *Block {
	return &Block{
		Network:       NETWORK,
		Height:        height,
		Timestamp:     time.Now().Unix(),
		PrevBlockHash: prevBlockHash,
		TrieHash:      nil,
		SheetHash:     prevSheetHash,

		Difficulty: diff.Bytes(),
		Nonce:      nil,

		Sheet: nil,
		Txs:   make([]*Tx, 0),
	}
}

// CheckError will check the errors in block inner fields.
func (x *Block) CheckError() error {
	if x.Network != NETWORK {
		return fmt.Errorf("block's network id is incorrect")
	}

	if !x.IsSealed() {
		return fmt.Errorf("block@%d has not sealed with nonce", x.GetHeight())
	}

	if !bytes.Equal(NewTxTrie(x.Txs).TrieRoot(), x.GetTrieHash()) {
		return fmt.Errorf("the merkle tree in block@%d is invalid", x.GetHeight())
	}

	err := x.verifyNonce()
	if err != nil {
		return err
	}

	return nil
}

// Hash will help you get the hash of block.
func (x *Block) Hash() []byte {
	b := proto.Clone(x).(*Block)
	b.Sheet = nil
	b.Txs = nil

	raw, err := utils.Proto.Marshal(x)
	if err != nil {
		panic(err)
	}

	hash := sha3.Sum256(raw)

	return hash[:]
}

// GetPrevHash is a helper to get the prev block hash from block header.
func (x *Block) GetPrevHash() []byte {
	return x.GetPrevBlockHash()
}

var GenesisBlock *Block
var GenesisBlockHash []byte

// GetGenesisBlock will return a complete sealed GenesisBlock.
func init() {
	txs := []*Tx{
		GetGenesisGenerateTx(),
	}

	GenesisBlock = &Block{
		Network:   NETWORK,
		Height:    0,
		Timestamp: genesisTimestamp,

		PrevBlockHash: nil,
		TrieHash:      NewTxTrie(txs).TrieRoot(),
		SheetHash:     nil,

		Difficulty: minimumBigDifficulty.Bytes(),
		Nonce:      genesisBlockNonce.Bytes(),
		Sheet:      GenesisSheet,
		Txs:        txs,
	}

	GenesisBlockHash = GenesisBlock.Hash()
}
