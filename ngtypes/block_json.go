package ngtypes

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/utils"
)

type jsonBlockHeader struct {
	Height        uint64
	Timestamp     int64
	PrevBlockHash string
	TrieHash      string
	Difficulty    string
	Nonce         string
}

func (x BlockHeader) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonBlockHeader{
		Height:        x.GetHeight(),
		Timestamp:     x.GetTimestamp(),
		PrevBlockHash: hex.EncodeToString(x.GetPrevBlockHash()),
		TrieHash:      hex.EncodeToString(x.GetTrieHash()),
		Difficulty:    new(big.Int).SetBytes(x.GetDifficulty()).String(),
		Nonce:         hex.EncodeToString(x.GetNonce()),
	})
}

func (x *BlockHeader) UnmarshalJSON(b []byte) error {
	var header jsonBlockHeader
	err := utils.JSON.Unmarshal(b, &header)
	if err != nil {
		return err
	}

	x.Height = header.Height

	x.Timestamp = int64(header.Timestamp)

	x.PrevBlockHash, err = hex.DecodeString(header.PrevBlockHash)
	if err != nil {
		return err
	}

	x.TrieHash, err = hex.DecodeString(header.TrieHash)
	if err != nil {
		return err
	}

	bigDifficulty, ok := new(big.Int).SetString(header.Difficulty, 10)
	if !ok {
		return fmt.Errorf("failed to parse blockHeader's difficulty")
	}
	x.Difficulty = bigDifficulty.Bytes()

	x.Nonce, err = hex.DecodeString(header.Nonce)
	if err != nil {
		return err
	}

	return nil
}
