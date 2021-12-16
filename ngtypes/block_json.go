package ngtypes

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ngchain/ngcore/utils"
)

type jsonBlock struct {
	Network string `json:"network"`

	Height    uint64 `json:"height"`
	Timestamp uint64 `json:"timestamp"`

	PrevBlockHash string `json:"prevBlockHash"`
	TxTrieHash    string `json:"txTrieHash"`
	SubTrieHash   string `json:"subTrieHash"`

	Difficulty string `json:"difficulty"`
	Nonce      string `json:"nonce"`

	Txs []*FullTx `json:"txs"`

	// some helper fields
	Hash    string `json:"hash,omitempty"`
	PoWHash string `json:"powHash,omitempty"`
	Txn     int    `json:"txn,omitempty"`
}

// MarshalJSON encodes the Block into the json bytes
func (x *FullBlock) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonBlock{
		Network:       x.BlockHeader.Network.String(),
		Height:        x.BlockHeader.Height,
		Timestamp:     x.BlockHeader.Timestamp,
		PrevBlockHash: hex.EncodeToString(x.BlockHeader.PrevBlockHash),
		TxTrieHash:    hex.EncodeToString(x.BlockHeader.TxTrieHash),
		SubTrieHash:   hex.EncodeToString(x.BlockHeader.SubTrieHash),
		Difficulty:    new(big.Int).SetBytes(x.BlockHeader.Difficulty).String(),
		Nonce:         hex.EncodeToString(x.BlockHeader.Nonce),
		Txs:           x.Txs,

		Hash:    hex.EncodeToString(x.GetHash()),
		PoWHash: hex.EncodeToString(x.PowHash()),
		Txn:     len(x.Txs),
	})
}

// ErrInvalidDiff means the diff cannot load from the string
var ErrInvalidDiff = errors.New("failed to parse blockHeader's difficulty")

// UnmarshalJSON decode the Block from the json bytes
func (x *FullBlock) UnmarshalJSON(data []byte) error {
	var b jsonBlock
	err := utils.JSON.Unmarshal(data, &b)
	if err != nil {
		return err
	}

	prevBlockHash, err := hex.DecodeString(b.PrevBlockHash)
	if err != nil {
		return err
	}
	txTrieHash, err := hex.DecodeString(b.TxTrieHash)
	if err != nil {
		return err
	}
	subTrieHash, err := hex.DecodeString(b.SubTrieHash)
	if err != nil {
		return err
	}
	bigDifficulty, ok := new(big.Int).SetString(b.Difficulty, 10)
	if !ok {
		return ErrInvalidDiff
	}
	difficulty := bigDifficulty.Bytes()
	nonce, err := hex.DecodeString(b.Nonce)
	if err != nil {
		return err
	}

	*x = *NewBlock(
		GetNetwork(b.Network),
		b.Height,
		b.Timestamp,
		prevBlockHash,
		txTrieHash,
		subTrieHash,
		difficulty,
		nonce,
		b.Txs,
		[]*BlockHeader{}, // TODO
	)

	// err = x.verifyNonce()
	// if err != nil {
	//	return err
	// }

	return nil
}
