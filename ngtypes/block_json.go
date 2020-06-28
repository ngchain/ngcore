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
	//PrevSheetHash string `json:"prevSheetHash"`
	Difficulty string `json:"difficulty"`
	Nonce      string `json:"nonce"`

	//PrevSheet *Sheet `json:"prevSheet"`
	Txs []*Tx `json:"txs"`

	// some helper fields
	Hash    string `json:"hash"`
	PoWHash string `json:"powHash"`
	Txn     int    `json:"txn"`
}

func (x *Block) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonBlock{
		Network:       int(x.Network),
		Height:        x.GetHeight(),
		Timestamp:     x.GetTimestamp(),
		PrevBlockHash: hex.EncodeToString(x.GetPrevBlockHash()),
		TrieHash:      hex.EncodeToString(x.GetTrieHash()),
		Difficulty:    new(big.Int).SetBytes(x.GetDifficulty()).String(),
		Nonce:         hex.EncodeToString(x.GetNonce()),
		Txs:           x.GetTxs(),

		Hash:    hex.EncodeToString(x.Hash()),
		PoWHash: hex.EncodeToString(x.PowHash()),
		Txn:     len(x.Txs),
	})
}

func (x *Block) UnmarshalJSON(data []byte) error {
	var b jsonBlock
	err := utils.JSON.Unmarshal(data, &b)
	if err != nil {
		return err
	}

	network := NetworkType(b.Network)

	prevBlockHash, err := hex.DecodeString(b.PrevBlockHash)
	if err != nil {
		return err
	}
	trieHash, err := hex.DecodeString(b.TrieHash)
	if err != nil {
		return err
	}
	bigDifficulty, ok := new(big.Int).SetString(b.Difficulty, 10)
	if !ok {
		return fmt.Errorf("failed to parse blockHeader's difficulty")
	}
	difficulty := bigDifficulty.Bytes()
	nonce, err := hex.DecodeString(b.Nonce)
	if err != nil {
		return err
	}

	*x = Block{
		Network:       network,
		Height:        b.Height,
		Timestamp:     b.Timestamp,
		PrevBlockHash: prevBlockHash,
		TrieHash:      trieHash,
		Difficulty:    difficulty,
		Nonce:         nonce,
		Txs:           b.Txs,
	}

	return nil
}
