package miner_test

import (
	miner "github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
	"math/big"
	"testing"
	"time"
)

func TestPoWMiner(t *testing.T) {
	block := ngtypes.GetGenesisBlock(ngtypes.NetworkType_TESTNET)

	block.Difficulty = big.NewInt(100).Bytes() // lower for avoid timeout

	ch := make(chan *ngtypes.Block)
	m := miner.NewMiner(2, ch)

	go func() {
		result := <-ch
		if err := result.CheckError(); err != nil {
			panic(err)
		}
	}()

	m.Start(block)
	time.Sleep(time.Minute)

	m.Stop()
}
