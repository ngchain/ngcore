package models

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
)

func NewAccount(a *ngtypes.Account) *Account {
	return &Account{
		Num:      int64(a.Num),
		Owner:    base58.FastBase58Encoding(a.Owner),
		Nonce:    int64(a.Nonce),
		Contract: hex.EncodeToString(a.Contract),
		State:    hex.EncodeToString(a.State),
	}
}

func NewTx(tx *ngtypes.Tx) *Tx {
	participants := make([]string, len(tx.Header.Participants))
	for i, v := range tx.Header.Participants {
		participants[i] = base58.FastBase58Encoding(v)
	}

	values := make([]float64, len(tx.Header.Values))
	for i, v := range tx.Header.Values {
		values[i] = float64(new(big.Int).SetBytes(v).Uint64()) / ngtypes.FloatNG
	}

	return &Tx{
		Header: &TxHeader{
			Convener:     int64(tx.Header.Convener),
			Extra:        hex.EncodeToString(tx.Header.Extra),
			Fee:          float64(new(big.Int).SetBytes(tx.Header.Fee).Uint64()) / ngtypes.FloatNG,
			Nonce:        int64(tx.Header.Nonce),
			Participants: participants,
			Type:         int64(tx.Header.Type),
			Values:       values,
		},
		Network: Network(tx.Network),
		Sign:    hex.EncodeToString(tx.Sign),
	}
}

func NewSheet(sheet *ngtypes.Sheet) *Sheet {
	accounts := make(map[string]Account)
	for k, v := range sheet.Accounts {
		accounts[strconv.FormatUint(k, 10)] = Account{
			Num:      int64(v.Num),
			Owner:    base58.FastBase58Encoding(v.Owner),
			Nonce:    int64(v.Nonce),
			Contract: hex.EncodeToString(v.Contract),
			State:    hex.EncodeToString(v.State),
		}
	}

	anonymous := make(map[string]float64)
	for k, v := range sheet.Anonymous {
		anonymous[k] = float64(new(big.Int).SetBytes(v).Uint64()) / ngtypes.FloatNG
	}

	return &Sheet{
		Accounts:  accounts,
		Anonymous: anonymous,
	}
}

func NewBlock(block *ngtypes.Block) *Block {
	txs := make([]*Tx, len(block.Txs))
	for i, v := range block.Txs {
		txs[i] = NewTx(v)
	}

	return &Block{
		Header: &BlockHeader{
			Height:        int64(block.GetHeight()),
			Nonce:         hex.EncodeToString(block.Header.Nonce),
			PrevBlockHash: hex.EncodeToString(block.Header.PrevBlockHash),
			SheetHash:     hex.EncodeToString(block.Header.SheetHash),
			Difficulty:    hex.EncodeToString(block.Header.Difficulty),
			TrieHash:      hex.EncodeToString(block.Header.TrieHash),
			Timestamp:     block.Header.Timestamp,
		},
		Network: Network(block.Network),
		Sheet:   NewSheet(block.Sheet),
		Txs:     txs,
	}
}
