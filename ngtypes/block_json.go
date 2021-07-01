package ngtypes

import (
	"encoding/hex"
	"fmt"
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

	Txs []*Tx `json:"txs"`

	// some helper fields
	Hash    string `json:"hash,omitempty"`
	PoWHash string `json:"powHash,omitempty"`
	Txn     int    `json:"txn,omitempty"`
}

func (x *Block) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonBlock{
		Network:       x.Header.Network.String(),
		Height:        x.Header.Height,
		Timestamp:     x.Header.Timestamp,
		PrevBlockHash: hex.EncodeToString(x.Header.PrevBlockHash),
		TxTrieHash:    hex.EncodeToString(x.Header.TxTrieHash),
		SubTrieHash:   hex.EncodeToString(x.Header.SubTrieHash),
		Difficulty:    new(big.Int).SetBytes(x.Header.Difficulty).String(),
		Nonce:         hex.EncodeToString(x.Header.Nonce),
		Txs:           x.GetTxs(),

		Hash:    hex.EncodeToString(x.GetHash()),
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
		return fmt.Errorf("failed to parse blockHeader's difficulty")
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
