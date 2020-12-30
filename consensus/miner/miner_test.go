package miner_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestPoWMiner(t *testing.T) {
	block := ngtypes.GetGenesisBlock(ngtypes.NetworkType_TESTNET)

	block.Difficulty = big.NewInt(100).Bytes() // lower for avoid timeout

	ch := make(chan *ngtypes.Block)
	m := miner.NewMiner(1, ch)

	go func() {
		for {
			result := <-ch
			if err := result.CheckError(); err != nil {
				panic(err)
			}
		}
	}()

	m.Mine(block)
	time.Sleep(time.Second)

	m.Stop() // should stop and exit
	t.Log(m.Job)
}
