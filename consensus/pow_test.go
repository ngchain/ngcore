package consensus_test

import (
	"testing"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

func TestNewConsensusManager(t *testing.T) {
	key := keytools.NewLocalKey()

	db := storage.InitMemStorage()

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := storage.NewChain(db)
	chain.InitWithGenesis()

	_ = ngp2p.NewLocalNode(52520)

	m := ngstate.GetStateManager()
	err := m.UpgradeState(ngtypes.GetGenesisBlock())
	if err != nil {
		panic(err)
	}

	_ = consensus.NewPoWConsensus(1, key, true)
}
