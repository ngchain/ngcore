package jsonrpc_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// TODO: add tests for each method rather than testing the server.
func TestNewRPCServer(t *testing.T) {
	network := ngtypes.ZERONET

	db := storage.InitTempStorage()
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	store := ngblocks.Init(db, network)
	state := ngstate.InitStateFromGenesis(db, network)

	chain := blockchain.Init(db, network, store, state)
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
		},
	)
	pow.GoLoop()

	rpc := jsonrpc.NewServer(pow, jsonrpc.ServerConfig{
		Host:                 "",
		Port:                 52520,
		DisableP2PMethods:    false,
		DisableMiningMethods: false,
	})
	go rpc.Serve()

	go func() {
		finished := time.After(2 * time.Minute)

		for {
			<-finished

			return
		}
	}()
}
