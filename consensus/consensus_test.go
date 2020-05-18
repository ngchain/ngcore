package consensus_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

func TestNewConsensusManager(t *testing.T) {
	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(""))
	keytools.PrintPublicKey(key)

	db := storage.InitMemStorage()

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := storage.NewChain(db)
	chain.InitWithGenesis()

	_ = ngp2p.NewLocalNode(rand.Int())

	s, err := ngstate.NewStateFromSheet(ngtypes.GetGenesisSheet())
	if err != nil {
		panic(err)
	}

	_ = txpool.NewTxPool(s)

	consensus.NewPoWConsensus(1, key, false)
}
