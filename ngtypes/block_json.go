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
	WitnessRoot   string `json:"subTrieHash"`
	StateRoot     string `json:"stateRoot"`

	Difficulty string `json:"difficulty"`
	Nonce      string `json:"nonce"`

	Txs []*FullTx `json:"txs"`

	UnclesHash string      `json:"unclesHash"`
	Uncles     []jsonUncle `json:"uncles,omitempty"`

	// some helper fields
	Hash    string `json:"hash,omitempty"`
	PoWHash string `json:"powHash,omitempty"`
	Txn     int    `json:"txn,omitempty"`
}

// jsonUncle is the read-only view of a referenced uncle header
type jsonUncle struct {
	Hash       string `json:"hash"`
	Height     uint64 `json:"height"`
	Difficulty string `json:"difficulty"`
}

// MarshalJSON encodes the Block into the json bytes
func (x *FullBlock) MarshalJSON() ([]byte, error) {
	var uncles []jsonUncle
	for _, u := range x.Uncles {
		uncles = append(uncles, jsonUncle{
			Hash:       hex.EncodeToString(u.GetHash()),
			Height:     u.Height,
			Difficulty: new(big.Int).SetBytes(u.Difficulty).String(),
		})
	}

	return utils.JSON.Marshal(jsonBlock{
		Network:       x.BlockHeader.Network.String(),
		Height:        x.BlockHeader.Height,
		Timestamp:     x.BlockHeader.Timestamp,
		PrevBlockHash: hex.EncodeToString(x.BlockHeader.PrevBlockHash),
		TxTrieHash:    hex.EncodeToString(x.BlockHeader.TxTrieHash),
		WitnessRoot:   hex.EncodeToString(x.BlockHeader.WitnessRoot),
		StateRoot:     hex.EncodeToString(x.BlockHeader.StateRoot),
		Difficulty:    new(big.Int).SetBytes(x.BlockHeader.Difficulty).String(),
		Nonce:         hex.EncodeToString(x.BlockHeader.Nonce),
		Txs:           x.Txs,

		UnclesHash: hex.EncodeToString(x.BlockHeader.UnclesHash),
		Uncles:     uncles,

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
	subTrieHash, err := hex.DecodeString(b.WitnessRoot)
	if err != nil {
		return err
	}
	stateRoot, err := hex.DecodeString(b.StateRoot)
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

	network, err := ParseNetwork(b.Network)
	if err != nil {
		return err
	}

	*x = *NewBlock(
		network,
		b.Height,
		b.Timestamp,
		prevBlockHash,
		txTrieHash,
		subTrieHash,
		difficulty,
		nonce,
		b.Txs,
	)
	// NewBlock zeroes StateRoot; carry the decoded one so a round-trip
	// preserves the committed post-state root (empty stays zero-length)
	if len(stateRoot) > 0 {
		x.BlockHeader.StateRoot = stateRoot
	}

	// err = x.verifyNonce()
	// if err != nil {
	//	return err
	// }

	return nil
}
