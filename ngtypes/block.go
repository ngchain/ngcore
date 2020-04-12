package ngtypes

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/ngchain/cryptonight-go"
	"github.com/whyrusleeping/go-logging"
)

// all block strcuture inner errors
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
	ErrBlockMalformed         = errors.New("the block structure is malformed")
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
func (m *Block) ToUnsealing(txsWithGen []*Tx) (*Block, error) {
	if m.Header == nil {
		return nil, ErrBlockHeaderMissing
	}

	for i := 0; i < len(txsWithGen); i++ {
		if i == 0 && txsWithGen[i].GetType() != TX_GENERATE {
			return nil, fmt.Errorf("first tx shall be a generate")
		}

		if i != 0 && txsWithGen[i].GetType() == TX_GENERATE {
			return nil, fmt.Errorf("except first, other tx shall not be a generate")
		}
	}

	m.Header.TrieHash = NewTxTrie(txsWithGen).TrieRoot()
	m.Txs = txsWithGen

	return m, nil
}

// ToSealed converts an unsealing block to a sealed block
func (m *Block) ToSealed(nonce []byte) (*Block, error) {
	if m.Header == nil {
		return nil, ErrBlockHeaderMissing
	}

	if !m.Header.IsUnsealing() {
		return nil, ErrBlockIsBare
	}

	m.Header.Nonce = nonce

	return m, nil
}

// VerifyNonce will verify whether the nonce meets the target
func (m *Block) VerifyNonce() error {
	if new(big.Int).SetBytes(cryptonight.Sum(m.Header.GetPoWBlob(nil), 0)).Cmp(new(big.Int).SetBytes(m.Header.Target)) < 0 {
		return nil
	}

	return ErrBlockNonceInvalid
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N
func NewBareBlock(height uint64, prevBlockHash []byte, target *big.Int) *Block {
	return &Block{
		NetworkId: NetworkID,
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

// GetGenesisBlock will return a complete sealed GenesisBlock
func GetGenesisBlock() *Block {
	txs := []*Tx{
		GetGenesisGenerateTx(),
	}

	header := &BlockHeader{
		Height:        0,
		Timestamp:     1500000000,
		PrevBlockHash: nil,
		SheetHash:     GenesisSheetHash,
		TrieHash:      NewTxTrie(txs).TrieRoot(),
		Target:        GenesisTarget.Bytes(),
		Nonce:         GenesisNonce.Bytes(),
	}

	return &Block{
		NetworkId: NetworkID,
		Header:    header,
		Sheet:     GetGenesisSheet(),
		Txs:       txs,
	}
}

// CheckError will check the errors in block inner fields
func (m *Block) CheckError() error {
	if m.NetworkId != NetworkID {
		return fmt.Errorf("block's network id is incorrect")
	}

	if m.Header == nil {
		return ErrBlockHeaderMissing
	}

	if m.Header.Nonce == nil {
		return ErrBlockNonceInvalid
	}

	if !bytes.Equal(NewTxTrie(m.Txs).TrieRoot(), m.Header.TrieHash) {
		return ErrBlockMTreeInvalid
	}

	if err := m.VerifyNonce(); err != nil {
		return err
	}

	return nil
}

// CalculateHash will help you get the hash of block
func (m *Block) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}

// GetHeight is a helper to get the height from block header
func (m *Block) GetHeight() uint64 {
	return m.Header.Height
}

// GetPrevHash is a helper to get the prev block hash from block header
func (m *Block) GetPrevHash() []byte {
	return m.Header.PrevBlockHash
}

// GenesisBlockHash is a helper to get the genesis block's hash
var GenesisBlockHash, _ = GetGenesisBlock().CalculateHash()
