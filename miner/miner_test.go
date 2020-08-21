package miner_test

import (
	miner "github.com/ngchain/ngcore/miner"
	"github.com/ngchain/ngcore/ngtypes"
	"math/big"
	"testing"
)

func TestPoWMiner(t *testing.T) {
	block := ngtypes.GetGenesisBlock()

	block.Difficulty = big.NewInt(100).Bytes() // lower for avoid timeout

	ch := make(chan *ngtypes.Block)
	m := miner.NewMiner(2, ch)

	m.Start(block)
	result := <-ch
	if err := result.CheckError(); err != nil {
		panic(err)
	}

	m.Stop()
}
