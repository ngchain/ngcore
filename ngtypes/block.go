package ngtypes

import (
	"bytes"
	"errors"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"
	"math/big"
	"time"

	"github.com/ngin-network/cryptonight-go"
	"github.com/whyrusleeping/go-logging"
)

var (
	ErrBlockHeaderMissing     = errors.New("the block's header is missing")
	ErrBlockHeaderHashMissing = errors.New("the block's header hash is missing")
	ErrBlockIsBare            = errors.New("the block is bare")
	ErrBlockIsUnsealing       = errors.New("the block is unsealing")
	ErrBlockHeightInvalid     = errors.New("the block's height is invalid")
	ErrBlockMTreeInvalid      = errors.New("the merkle tree in block is invalid")
	ErrBlockPrevBlockHash     = errors.New("the block's previous block hash is invalid")
	ErrBlockPrevTreasuryHash  = errors.New("the block's backend vault is invalid")
	ErrBlockDiffInvalid       = errors.New("the block's difficulty is invalid")
	ErrBlockHashInvalid       = errors.New("the block's hash is invalid")
	ErrBlockNonceInvalid      = errors.New("the block's N is invalid")
	ErrMalformedBlock         = errors.New("the block structure is malformed")
)

var log = logging.MustGetLogger("types")

/* Body Start */

func (m *Block) IsUnsealing() (bool, error) {
	if m.Header != nil {
		return false, ErrBlockHeaderMissing
	}
	return m.Header.IsUnsealing(), nil
}

func (m *Block) IsSealed() (bool, error) {
	if m.Header != nil {
		return false, ErrBlockHeaderMissing
	}

	return m.Header.IsSealed(), nil
}

func (m *Block) IsHead() bool {
	return m.Header.IsHead()
}

// ToUnsealing converts a bare block to an unsealing block
func (m *Block) ToUnsealing(txsWithGen []*Transaction) (*Block, error) {
	if m.Header == nil {
		return nil, ErrBlockHeaderMissing
	}

	b := m.Copy()
	b.Header.TrieHash = NewTxTrie(txsWithGen).TrieRoot()
	b.Transactions = txsWithGen

	return b, nil
}

// ToUnsealing converts an unsealing block to a sealed block
func (m *Block) ToSealed(nonce []byte) (*Block, error) {
	if m.Header == nil {
		return nil, ErrBlockHeaderMissing
	}

	if !m.Header.IsUnsealing() {
		return nil, ErrBlockIsBare
	}

	b := m.Copy()
	b.Header.Nonce = nonce
	b.HeaderHash = cryptonight.Sum(b.Header.GetPoWBlob(nonce), 0)

	return b, nil
}

// CalculateHash will help you get the hash of block
func (m *Block) VerifyNonce() bool {
	return new(big.Int).SetBytes(cryptonight.Sum(m.Header.GetPoWBlob(nil), 0)).Cmp(new(big.Int).SetBytes(m.Header.Target)) < 0
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N
func NewBareBlock(height uint64, prevBlockHash, prevVaultHash []byte, target *big.Int) *Block {
	block := &Block{
		NetworkId: NetworkId,
		Header: &BlockHeader{
			Version:       Version,
			Height:        height,
			PrevBlockHash: prevBlockHash,
			PrevVaultHash: prevVaultHash,
			TrieHash:      nil,
			Timestamp:     time.Now().Unix(),
			Target:        target.Bytes(),
			Nonce:         nil,
		},
		Transactions: make([]*Transaction, 0),
	}

	return block
}

// GetGenesisBlock will return a complete sealed GenesisBlock
func GetGenesisBlock() *Block {
	txs := []*Transaction{
		GetGenesisGeneration(),
	}

	header := &BlockHeader{
		Timestamp:     1024,
		TrieHash:      NewTxTrie(txs).TrieRoot(),
		PrevBlockHash: nil,
		PrevVaultHash: GenesisVaultHash,
		Nonce:         GenesisNonce.Bytes(),
		Target:        GenesisTarget.Bytes(),
	}

	hash := header.CalculateHash()

	return &Block{
		Header:       header,
		HeaderHash:   hash,
		Transactions: txs,
	}
}

// CheckError will check the errors in block inner fields
func (m *Block) CheckError() error {
	if m.Header == nil {
		return ErrBlockHeaderMissing
	}

	if m.Header.Nonce == nil {
		return ErrBlockNonceInvalid
	}

	if m.HeaderHash == nil {
		return ErrBlockHeaderHashMissing
	}

	mTreeHash := NewTxTrie(m.Transactions).TrieRoot()
	if bytes.Compare(mTreeHash, m.Header.TrieHash) != 0 {
		return ErrBlockMTreeInvalid
	}

	return nil
}

func (m *Block) Copy() *Block {
	b := proto.Clone(m).(*Block)
	return b
}

func (m *Block) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}

func (m *Block) GetHeight() uint64 {
	return m.Header.Height
}

func (m *Block) GetPrevHash() []byte {
	return m.Header.PrevBlockHash
}

var GenesisBlockHash, _ = GetGenesisBlock().CalculateHash()
