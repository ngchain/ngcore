package consensus_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/storage"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/secp256k1"
)

func TestPoWMiner(t *testing.T) {
	ngblocks.Init(storage.InitMemStorage())

	_ = ngp2p.NewLocalNode(52520)
	pk := secp256k1.NewPrivateKey(big.NewInt(1))
	consensus.InitPoWConsensus(1, pk, true) // as bootstrap to avoid sync
	consensus.MiningOn()
	time.Sleep(30 * time.Second)
	consensus.MiningOff()
}
