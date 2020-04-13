package ngtypes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/ngchain/cryptonight-go"
	"github.com/whyrusleeping/go-logging"
)

var log = logging.MustGetLogger("types")

func (m *Block) IsUnsealing() bool {
	return m.GetHeader().GetTrieHash() != nil
}

func (m *Block) IsSealed() bool {
	return m.GetHeader().GetNonce() != nil
}

// IsHead will check whether the Block is the checkpoint
func (m *Block) IsHead() bool {
	return m.GetHeight()%BlockCheckRound == 0
}

func (m *Block) IsTail() bool {
	return (m.GetHeight()+1)%BlockCheckRound == 0
}

func (m *Block) IsGenesis() bool {
	hash, _ := m.CalculateHash()
	return bytes.Equal(hash, GenesisBlockHash)
}

// GetPoWBlob will return a complete blob for block hash
func (m *Block) GetPoWBlob(nonce []byte) []byte {
	raw := make([]byte, 144)

	h := m.GetHeader()
	copy(raw[0:32], h.GetPrevBlockHash())
	copy(raw[32:64], h.GetSheetHash())
	copy(raw[64:96], h.GetTrieHash())
	binary.LittleEndian.PutUint64(raw[96:104], uint64(h.GetTimestamp()))
	copy(raw[104:136], h.GetTarget()) // uint256

	if nonce == nil {
		copy(raw[136:144], h.GetNonce()) // 8
	} else {
		copy(raw[136:144], nonce) // 8
	}

	return raw
}

// CalculateHeaderHash will help you get the hash of block
func (m *Block) CalculateHeaderHash() []byte {
	blob := m.GetPoWBlob(nil)
	return cryptonight.Sum(blob, 0)
}

// ToUnsealing converts a bare block to an unsealing block
func (m *Block) ToUnsealing(txsWithGen []*Tx) (*Block, error) {
	if m.GetHeader() == nil {
		return nil, fmt.Errorf("missing header")
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
	if m.GetHeader() == nil {
		return nil, fmt.Errorf("missing header")
	}

	if !m.IsUnsealing() {
		return nil, fmt.Errorf("the block is bare")
	}

	m.Header.Nonce = nonce

	return m, nil
}

// verifyNonce will verify whether the nonce meets the target
func (m *Block) verifyNonce() error {
	if new(big.Int).SetBytes(cryptonight.Sum(m.GetPoWBlob(nil), 0)).Cmp(new(big.Int).SetBytes(m.GetHeader().GetTarget())) < 0 {
		return nil
	}

	return fmt.Errorf("block@%d's nonce %x is invalid", m.GetHeight(), m.Header.GetNonce())
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

	if m.GetHeader() == nil {
		return fmt.Errorf("missing header")
	}

	if !m.IsSealed() {
		return fmt.Errorf("block@%d has not sealed with nonce", m.GetHeight())
	}

	if !bytes.Equal(NewTxTrie(m.Txs).TrieRoot(), m.GetHeader().GetTrieHash()) {
		return fmt.Errorf("the merkle tree in block@%d is invalid", m.GetHeight())
	}

	if err := m.verifyNonce(); err != nil {
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
	return m.GetHeader().GetHeight()
}

// GetPrevHash is a helper to get the prev block hash from block header
func (m *Block) GetPrevHash() []byte {
	return m.GetHeader().GetPrevBlockHash()
}

// GenesisBlockHash is a helper to get the genesis block's hash
var GenesisBlockHash, _ = GetGenesisBlock().CalculateHash()
