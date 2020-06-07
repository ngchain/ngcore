package ngtypes

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/utils"
)

type jsonBlock struct {
	Network int `json:"network"`

	Height        uint64 `json:"height"`
	Timestamp     int64  `json:"timestamp"`
	PrevBlockHash string `json:"prevBlockHash"`
	TrieHash      string `json:"trieHash"`
	PrevSheetHash string `json:"prevSheetHash"`
	Difficulty    string `json:"difficulty"`
	Nonce         string `json:"nonce"`

	PrevSheet *Sheet `json:"prevSheet"`
	Txs       []*Tx  `json:"txs"`
}

func (x *Block) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonBlock{
		Network:       int(x.Network),
		Height:        x.GetHeight(),
		Timestamp:     x.GetTimestamp(),
		PrevBlockHash: hex.EncodeToString(x.GetPrevBlockHash()),
		TrieHash:      hex.EncodeToString(x.GetTrieHash()),
		PrevSheetHash: hex.EncodeToString(x.GetPrevSheetHash()),
		Difficulty:    new(big.Int).SetBytes(x.GetDifficulty()).String(),
		Nonce:         hex.EncodeToString(x.GetNonce()),
		PrevSheet:     x.GetPrevSheet(),
		Txs:           x.GetTxs(),
	})
}

func (x *Block) UnmarshalJSON(data []byte) error {
	var b jsonBlock
	err := utils.JSON.Unmarshal(data, &b)
	if err != nil {
		return err
	}

	x.Network = NetworkType(b.Network)

	x.Height = b.Height
	x.Timestamp = int64(b.Timestamp)
	x.PrevBlockHash, err = hex.DecodeString(b.PrevBlockHash)
	if err != nil {
		return err
	}
	x.TrieHash, err = hex.DecodeString(b.TrieHash)
	if err != nil {
		return err
	}
	x.PrevSheetHash, err = hex.DecodeString(b.PrevSheetHash)
	if err != nil {
		return err
	}
	bigDifficulty, ok := new(big.Int).SetString(b.Difficulty, 10)
	if !ok {
		return fmt.Errorf("failed to parse blockHeader's difficulty")
	}
	x.Difficulty = bigDifficulty.Bytes()
	x.Nonce, err = hex.DecodeString(b.Nonce)
	if err != nil {
		return err
	}

	x.PrevSheet = b.PrevSheet
	x.Txs = b.Txs

	return nil
}
