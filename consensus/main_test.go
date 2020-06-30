package consensus_test

import (
	"strings"
	"testing"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func TestNewConsensusManager(t *testing.T) {
	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(""))
	keytools.PrintAddress(key)

	db := storage.InitMemStorage()

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := storage.NewChain(db)
	chain.InitWithGenesis()

	_ = ngp2p.NewLocalNode(int(utils.RandUint16()))

	m := ngstate.GetStateManager()
	m.UpdateState(ngtypes.GetGenesisBlock())

	_ = consensus.NewPoWConsensus(1, key, true)
}
