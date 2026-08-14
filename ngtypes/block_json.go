package ngtypes

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ngchain/ngcore/utils"
)

// jsonBlockHeader is the hex-encoded wire form of one sub header
type jsonBlockHeader struct {
	Network       string `json:"network"`
	Height        uint64 `json:"height"`
	Timestamp     uint64 `json:"timestamp"`
	PrevBlockHash string `json:"prevBlockHash"`
	TxTrieHash    string `json:"txTrieHash"`
	SubTrieHash   string `json:"subTrieHash"`
	Difficulty    string `json:"difficulty"`
	Nonce         string `json:"nonce"`
}

func headerToJSON(h *BlockHeader) jsonBlockHeader {
	return jsonBlockHeader{
		Network:       h.Network.String(),
		Height:        h.Height,
		Timestamp:     h.Timestamp,
		PrevBlockHash: hex.EncodeToString(h.PrevBlockHash),
		TxTrieHash:    hex.EncodeToString(h.TxTrieHash),
		SubTrieHash:   hex.EncodeToString(h.SubTrieHash),
		Difficulty:    new(big.Int).SetBytes(h.Difficulty).String(),
		Nonce:         hex.EncodeToString(h.Nonce),
	}
}

func headerFromJSON(j jsonBlockHeader) (*BlockHeader, error) {
	prev, err := hex.DecodeString(j.PrevBlockHash)
	if err != nil {
		return nil, err
	}
	txTrie, err := hex.DecodeString(j.TxTrieHash)
	if err != nil {
		return nil, err
	}
	subTrie, err := hex.DecodeString(j.SubTrieHash)
	if err != nil {
		return nil, err
	}
	diff, ok := new(big.Int).SetString(j.Difficulty, 10)
	if !ok {
		return nil, ErrInvalidDiff
	}
	nonce, err := hex.DecodeString(j.Nonce)
	if err != nil {
		return nil, err
	}

	return &BlockHeader{
		Network:       GetNetwork(j.Network),
		Height:        j.Height,
		Timestamp:     j.Timestamp,
		PrevBlockHash: prev,
		TxTrieHash:    txTrie,
		SubTrieHash:   subTrie,
		Difficulty:    diff.Bytes(),
		Nonce:         nonce,
	}, nil
}

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

	SubHeaders []jsonBlockHeader `json:"subHeaders,omitempty"`

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
		SubHeaders:    subHeadersToJSON(x.Subs),

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

	subHeaders := make([]*BlockHeader, 0, len(b.SubHeaders))
	for _, jh := range b.SubHeaders {
		h, err := headerFromJSON(jh)
		if err != nil {
			return err
		}
		subHeaders = append(subHeaders, h)
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
		subHeaders,
	)

	// err = x.verifyNonce()
	// if err != nil {
	//	return err
	// }

	return nil
}

func subHeadersToJSON(headers []*BlockHeader) []jsonBlockHeader {
	if len(headers) == 0 {
		return nil
	}

	out := make([]jsonBlockHeader, len(headers))
	for i, h := range headers {
		out[i] = headerToJSON(h)
	}

	return out
}
