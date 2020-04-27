package ngtypes

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	return x.GetHeader().GetTrieHash() != nil
}

// IsSealed checks whether the block is sealed.
func (x *Block) IsSealed() bool {
	return x.GetHeader().GetNonce() != nil
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
	hash, _ := x.CalculateHash()
	return bytes.Equal(hash, GetGenesisBlockHash())
}

// GetPoWBlob will return a complete blob for block hash.
func (x *Block) GetPoWBlob(nonce []byte) []byte {
	h := x.GetHeader()

	lenWithoutNonce := len(h.GetPrevBlockHash()) + len(h.GetSheetHash()) +
		len(h.GetTrieHash()) + 2<<3 + len(h.GetTarget())
	raw := make([]byte, lenWithoutNonce+NonceSize)

	copy(raw[0:32], h.GetPrevBlockHash())
	copy(raw[32:64], h.GetSheetHash())
	copy(raw[64:96], h.GetTrieHash())
	binary.LittleEndian.PutUint64(raw[96:104], uint64(h.GetTimestamp()))
	copy(raw[104:136], h.GetTarget()) // uint256

	if nonce == nil {
		copy(raw[lenWithoutNonce:], h.GetNonce())
	} else {
		copy(raw[lenWithoutNonce:], nonce)
	}

	return raw
}

// CalculateHeaderHash will help you get the hash of block.
func (x *Block) CalculateHeaderHash() []byte {
	blob := x.GetPoWBlob(nil)
	return cryptonight.Sum(blob, 0)
}

// ToUnsealing converts a bare block to an unsealing block.
func (x *Block) ToUnsealing(txsWithGen []*Tx) (*Block, error) {
	if x.GetHeader() == nil {
		return nil, fmt.Errorf("missing header")
	}

	if txsWithGen[0].GetType() != TxType_GENERATE {
		return nil, fmt.Errorf("first tx shall be a generate")
	}

	for i := 1; i < len(txsWithGen); i++ {
		if txsWithGen[i].GetType() == TxType_GENERATE {
			return nil, fmt.Errorf("except first, other tx shall not be a generate")
		}
	}

	x.Header.TrieHash = NewTxTrie(txsWithGen).TrieRoot()
	x.Txs = txsWithGen

	return x, nil
}

// ToSealed converts an unsealing block to a sealed block.
func (x *Block) ToSealed(nonce []byte) (*Block, error) {
	if x.GetHeader() == nil {
		return nil, fmt.Errorf("missing header")
	}

	if !x.IsUnsealing() {
		return nil, fmt.Errorf("the block is bare")
	}

	x.Header.Nonce = nonce

	return x, nil
}

// verifyNonce will verify whether the nonce meets the target.
func (x *Block) verifyNonce() error {
	if new(big.Int).SetBytes(x.CalculateHeaderHash()).Cmp(new(big.Int).SetBytes(x.GetHeader().GetTarget())) < 0 {
		return nil
	}

	return fmt.Errorf("block@%d's nonce %x is invalid", x.GetHeight(), x.Header.GetNonce())
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N.
func NewBareBlock(height uint64, prevBlockHash []byte, target *big.Int) *Block {
	return &Block{
		Network: Network,
		Header: &BlockHeader{
			Height:        height,
			Timestamp:     time.Now().Unix(),
			PrevBlockHash: prevBlockHash,
			SheetHash:     nil,
			TrieHash:      nil,
			Target:        target.Bytes(),
			Nonce:         nil,
		},
		Txs: make([]*Tx, 0),
	}
}

// GetGenesisBlock will return a complete sealed GenesisBlock.
func GetGenesisBlock() *Block {
	txs := []*Tx{
		GetGenesisGenerateTx(),
	}

	header := &BlockHeader{
		Height:        0,
		Timestamp:     genesisTimestamp,
		PrevBlockHash: nil,
		SheetHash:     GetGenesisSheetHash(),
		TrieHash:      NewTxTrie(txs).TrieRoot(),
		Target:        genesisTarget.Bytes(),
		Nonce:         genesisBlockNonce.Bytes(),
	}

	return &Block{
		Network: Network,
		Header:  header,
		Sheet:   GetGenesisSheet(),
		Txs:     txs,
	}
}

// CheckError will check the errors in block inner fields.
func (x *Block) CheckError() error {
	if x.Network != Network {
		return fmt.Errorf("block's network id is incorrect")
	}

	if x.GetHeader() == nil {
		return fmt.Errorf("missing header")
	}

	if !x.IsSealed() {
		return fmt.Errorf("block@%d has not sealed with nonce", x.GetHeight())
	}

	if !bytes.Equal(NewTxTrie(x.Txs).TrieRoot(), x.GetHeader().GetTrieHash()) {
		return fmt.Errorf("the merkle tree in block@%d is invalid", x.GetHeight())
	}

	if err := x.verifyNonce(); err != nil {
		return err
	}

	return nil
}

// CalculateHash will help you get the hash of block.
func (x *Block) CalculateHash() ([]byte, error) {
	raw, err := utils.Proto.Marshal(x)
	if err != nil {
		return nil, err
	}

	hash := sha3.Sum256(raw)

	return hash[:], nil
}

// GetHeight is a helper to get the height from block header.
func (x *Block) GetHeight() uint64 {
	return x.GetHeader().GetHeight()
}

// GetPrevHash is a helper to get the prev block hash from block header.
func (x *Block) GetPrevHash() []byte {
	return x.GetHeader().GetPrevBlockHash()
}

var genesisBlockHash []byte

// GetGenesisBlockHash is a helper to get the genesis block's hash.
func GetGenesisBlockHash() []byte {
	if genesisBlockHash == nil {
		genesisBlockHash, _ = GetGenesisBlock().CalculateHash()
	}

	return genesisBlockHash
}
