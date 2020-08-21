package miner_test

import (
	miner "github.com/ngchain/ngcore/miner"
	"github.com/ngchain/ngcore/ngtypes"
	"testing"
)

func TestPoWMiner(t *testing.T) {
	block := ngtypes.GetGenesisBlock()

	ch := make(chan *ngtypes.Block)
	m := miner.NewMiner(2, ch)

	m.Start(block)
	result := <-ch
	if err := result.CheckError(); err != nil {
		panic(err)
	}

	m.Stop()
}
