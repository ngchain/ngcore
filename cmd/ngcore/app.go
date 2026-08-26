package main

import (
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	// #nosec
	_ "net/http/pprof"

	logging "github.com/ngchain/zap-log"
	"github.com/urfave/cli/v2"
	"go.etcd.io/bbolt"

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

var nonStrictModeFlag = &cli.BoolFlag{
	Name: "non-strict",
	Usage: "Run as a light-verification node: may start from a remote " +
		"checkpoint and use snapshot (fast) sync, trusting the served " +
		"state sheets. The default strict mode verifies everything from " +
		"the genesis block",
}

var snapshotModeFlag = &cli.BoolFlag{
	Name:  "snapshot",
	Value: false,
	Usage: "Enable snapshot (fast) sync and converging; requires --non-strict",
}

var p2pTCPPortFlag = &cli.IntFlag{
	Name:  "p2p-port",
	Usage: "Port for P2P connection",
	Value: defaultTCPP2PPort,
}

var noDandelionFlag = &cli.BoolFlag{
	Name: "no-dandelion",
	Usage: "Disable Dandelion++ stem/fluff propagation: locally-submitted " +
		"txs and commitments flood the network immediately, giving up " +
		"network-origin privacy",
}

var rpcHostFlag = &cli.StringFlag{
	Name:  "rpc-host",
	Usage: "Host address for JSON RPC",
	Value: defaultRPCHost,
}

var rpcPortFlag = &cli.IntFlag{
	Name:  "rpc-port",
	Usage: "Port for JSON RPC",
	Value: defaultRPCPort,
}

var rpcDisableFlag = &cli.StringSliceFlag{
	Name:  "rpc-disable",
	Usage: "Disable some JSON RPC methods",
	Value: nil,
}

var isBootstrapFlag = &cli.BoolFlag{
	Name:  "bootstrap",
	Usage: "Enable starting local node as a bootstrap peer",
}

var testNetFlag = &cli.BoolFlag{
	Name:  "testnet",
	Usage: "Run node on the test network",
}

var zeroNetFlag = &cli.BoolFlag{
	Name:  "zeronet",
	Usage: "Run node on the regression test network",
}

var profileFlag = &cli.BoolFlag{
	Name:  "profile",
	Usage: "Enable writing cpu profile to the file",
}

var p2pKeyFileFlag = &cli.StringFlag{
	Name:  "p2p-key",
	Usage: "The file path to the p2p key",
	Value: "",
}

var minerExtraFlag = &cli.StringFlag{
	Name:  "miner-extra",
	Usage: "Extra data embedded in the generate tx of locally mined blocks",
}

var inMemFlag = &cli.BoolFlag{
	Name:  "in-mem",
	Usage: "Run the database of blocks, vaults in memory",
}

var pruneFlag = &cli.BoolFlag{
	Name: "prune",
	Usage: "Prune historical state: keep only the current state instead of " +
		"the default archive mode. Disables the height-parameter RPC reads " +
		"and makes reorgs replay instead of unwind",
}

var dbFolderFlag = &cli.StringFlag{
	Name:  "db-folder",
	Usage: "The folder location for db",
	Value: defaultDBFolder,
}

var log = logging.Logger("main")

var action = func(c *cli.Context) error {
	isBootstrapNode := c.Bool(isBootstrapFlag.Name)

	strictMode := isBootstrapNode || !c.Bool(nonStrictModeFlag.Name)
	snapshotMode := c.Bool(snapshotModeFlag.Name)

	// strict nodes verify every tx from the genesis block; snapshot sync
	// trusts remotely served state sheets — the two are incompatible
	if strictMode && snapshotMode {
		return errors.New("--snapshot requires --non-strict: snapshot sync trusts remote state sheets")
	}

	p2pTCPPort := c.Int(p2pTCPPortFlag.Name)
	rpcHost := c.String(rpcHostFlag.Name)
	rpcPort := c.Int(rpcPortFlag.Name)
	rpcDisables := c.StringSlice(rpcDisableFlag.Name)

	p2pKeyFile := c.String(p2pKeyFileFlag.Name)
	withProfile := c.Bool(profileFlag.Name)
	dbFolder := c.String(dbFolderFlag.Name)

	if !strictMode {
		log.Warn("running on non-strict mode")
	}

	network := ngtypes.TESTNET
	if c.Bool(testNetFlag.Name) {
		network = ngtypes.TESTNET
	}

	if c.Bool(zeroNetFlag.Name) {
		network = ngtypes.ZERONET // use zero net as the regression test network
	}

	if withProfile {
		go func() {
			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				panic(err)
			}
			log.Warnf("profiling on http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
			panic(http.Serve(listener, nil))
		}()
	}

	log.Warnf("ngcore version %s", Version)

	var db *bbolt.DB
	if c.Bool(inMemFlag.Name) {
		db = storage.InitTempStorage()
	} else {
		db = storage.InitStorage(network, dbFolder)
	}

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	store := ngblocks.Init(db, network)
	// then sync
	state := ngstate.InitStateFromGenesis(db, network)
	// archive is the default; --prune opts out of historical-state retention
	if c.Bool(pruneFlag.Name) {
		state.Archive = false
		log.Warn("running in prune mode: historical-state (height) reads are disabled")
	}

	chain := blockchain.Init(db, network, store, state)
	chain.CheckHealth(network)

	// upgrading a pre-archive db in place: rebuild the changeset history
	// once so historical reads and unwind work (no-op on fresh/covered dbs)
	if did, err := state.BackfillArchive(); err != nil {
		log.Panicf("archive backfill failed: %v", err)
	} else if did {
		log.Warn("archive backfill: rebuilt changeset history from the block store")
	}

	localNode := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:                  p2pKeyFile,
		Network:                     network,
		Port:                        p2pTCPPort,
		DisableDiscovery:            network == ngtypes.ZERONET,
		DisableConnectingBootstraps: isBootstrapNode || network == ngtypes.ZERONET,
		DisableDandelion:            c.Bool(noDandelionFlag.Name),
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
			StrictMode:                  strictMode,
			SnapshotMode:                snapshotMode,
			MinerExtraData:              []byte(c.String(minerExtraFlag.Name)),
			DisableConnectingBootstraps: isBootstrapNode || network == ngtypes.ZERONET,
		},
	)
	pow.GoLoop()

	// when rpcPort <= 0, disable rpc server
	if rpcPort > 0 {
		jsonRPCServerConfig := jsonrpc.ServerConfig{
			Host:                 rpcHost,
			Version:              Version,
			Port:                 rpcPort,
			DisableP2PMethods:    false,
			DisableMiningMethods: false,
		}

		for i := range rpcDisables {
			switch strings.ToLower(rpcDisables[i]) {
			case "p2p":
				jsonRPCServerConfig.DisableP2PMethods = true
			case "mining":
				jsonRPCServerConfig.DisableMiningMethods = true
			}
		}

		rpc := jsonrpc.NewServer(pow, jsonRPCServerConfig)
		go rpc.Serve()
	}

	// block until an exit signal arrives, then shut down cleanly:
	// consensus loops exit, the p2p host closes, and the deferred
	// db.Close() flushes the store
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Warnf("received %s, shutting down", sig)
	pow.Stop()

	return nil
}
