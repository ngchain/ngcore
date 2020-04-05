package ngtypes

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"

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

	for i := 0; i < len(txsWithGen); i++ {
		if i == 0 && txsWithGen[i].GetType() != TX_GENERATION {
			return nil, fmt.Errorf("first tx shall be a generation")
		}

		if i != 0 && txsWithGen[i].GetType() == TX_GENERATION {
			return nil, fmt.Errorf("except first, other tx shall not be a generation")
		}
	}

	b := m.Copy()
	b.Header.TrieHash = NewTxTrie(txsWithGen).TrieRoot()
	b.Transactions = txsWithGen

	return b, nil
}

// ToSealed converts an unsealing block to a sealed block
func (m *Block) ToSealed(nonce []byte) (*Block, error) {
	if m.Header == nil {
		return nil, ErrBlockHeaderMissing
	}

	if !m.Header.IsUnsealing() {
		return nil, ErrBlockIsBare
	}

	b := m.Copy()
	b.Header.Nonce = nonce

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
		NetworkId: NetworkID,
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

	return &Block{
		NetworkId:    NetworkID,
		Header:       header,
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

	mTreeHash := NewTxTrie(m.Transactions).TrieRoot()
	if !bytes.Equal(mTreeHash, m.Header.TrieHash) {
		return ErrBlockMTreeInvalid
	}

	m.Header.VerifyNonce()

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
