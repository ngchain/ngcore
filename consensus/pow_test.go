package consensus_test

import (
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngpool"
	"testing"

	"github.com/ngchain/ngcore/storage"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
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

	net := ngtypes.NetworkType_ZERONET
	store := ngblocks.Init(db, net)
	state := ngstate.InitStateFromGenesis(db, net)
	chain := ngchain.Init(db, net, store, nil)
	pool := ngpool.Init(db, chain)

	ngp2p.InitLocalNode(net, 52520, chain)

	consensus.InitPoWConsensus(db, chain, pool, state, consensus.PoWorkConfig{
		Network:                     net,
		DisableConnectingBootstraps: true,
		MiningThread:                1,
		PrivateKey:                  key,
	})
}
