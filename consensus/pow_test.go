package consensus_test

import (
	"github.com/ngchain/ngcore/ngtypes"
	"testing"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngpool"

	"github.com/ngchain/ngcore/storage"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
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

	net := ngtypes.ZERONET
	store := ngblocks.Init(db, net)
	state := ngstate.InitStateFromGenesis(db, net)
	chain := blockchain.Init(db, net, store, nil)

	localNode := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		Network:          net,
		Port:             52520,
		DisableDiscovery: true,
	})
	pool := ngpool.Init(db, chain, localNode)

	consensus.InitPoWConsensus(db, chain, pool, state, localNode, consensus.PoWorkConfig{
		Network:                     net,
		DisableConnectingBootstraps: true,
		MiningThread:                1,
		PrivateKey:                  key,
	})
}
