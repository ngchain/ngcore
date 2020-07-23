package consensus_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/secp256k1"
)

func TestPoWMiner(t *testing.T) {
	pk := secp256k1.NewPrivateKey(big.NewInt(0))
	pow := consensus.NewPoWConsensus(1, pk, false)
	pow.MiningOn()
	time.Sleep(30 * time.Second)
	pow.MiningOff()
}
