package ngtypes

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"google.golang.org/protobuf/proto"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
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
	Hash    string `json:"hash,omitempty"`
	PoWHash string `json:"powHash,omitempty"`
	Txn     int    `json:"txn,omitempty"`
}

func (x *Block) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonBlock{
		Network:       int(x.Header.GetNetwork()),
		Height:        x.Header.GetHeight(),
		Timestamp:     x.Header.GetTimestamp(),
		PrevBlockHash: hex.EncodeToString(x.Header.GetPrevBlockHash()),
		TrieHash:      hex.EncodeToString(x.Header.GetTrieHash()),
		Difficulty:    new(big.Int).SetBytes(x.Header.GetDifficulty()).String(),
		Nonce:         hex.EncodeToString(x.Header.GetNonce()),
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

	network := ngproto.NetworkType(b.Network)

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

	hash, err := hex.DecodeString(b.Hash)
	if err != nil {
		return err
	}

	*x = *NewBlock(
		network,
		b.Height,
		b.Timestamp,
		prevBlockHash,
		trieHash,
		difficulty,
		nonce,
		[]*ngproto.BlockHeader{}, // TODO
		b.Txs,
		hash,
	)

	err = x.verifyNonce()
	if err != nil {
		return err
	}

	err = x.verifyHash()
	if err != nil {
		return err
	}

	return nil
}

func (x *Block) Equals(other *Block) (bool, error) {
	if !proto.Equal(x.Header, other.Header) {
		return false, nil
	}
	if len(x.Txs) != len(other.Txs) {
		return false, nil
	}

	for i := 0; i < len(x.Txs); i++ {
		if eq, err := x.Txs[i].Equals(other.Txs[i]); !eq {
			return false, err
		}
	}

	if !bytes.Equal(x.Hash, other.Hash) {
		return false, nil
	}

	return true, nil
}
