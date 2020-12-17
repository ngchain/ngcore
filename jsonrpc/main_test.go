package jsonrpc_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// TODO: add tests for each method rather than testing the server
func TestNewRPCServer(t *testing.T) {
	network := ngtypes.NetworkType_ZERONET

	key := keytools.NewLocalKey()
	db := storage.InitMemStorage()
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	store := ngblocks.Init(db, network)
	state := ngstate.InitStateFromGenesis(db, network)

	chain := ngchain.Init(db, network, store, state)
	chain.CheckHealth(network)

	localNode := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:       "p2p.key",
		Network:          network,
		Port:             52520,
		DisableDiscovery: true,
	})
	localNode.GoServe()

	pool := ngpool.Init(db, chain, localNode)

	pow := consensus.InitPoWConsensus(
		db,
		chain,
		pool,
		state,
		localNode,
		consensus.PoWorkConfig{
			Network:                     network,
			DisableConnectingBootstraps: true,
			MiningThread:                -1,
			PrivateKey:                  key,
		},
	)
	pow.GoLoop()

	rpc := jsonrpc.NewServer("", 52521, pow)
	go rpc.Serve()

	go func() {
		finished := time.After(2 * time.Minute)
		for {
			<-finished
			return
		}
	}()
}
