package main

import (
	"net"
	"net/http"

	// #nosec
	_ "net/http/pprof"
	"strings"

	"github.com/c0mm4nd/dbolt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mr-tron/base58"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var nonStrictModeFlag = &cli.BoolFlag{
	Name: "non-strict",
	// Value: true, // local chain will be able to start from a checkpoint if false
	Usage: "Enable forcing ngcore starts from the genesis block",
}

var snapshotModeFlag = &cli.BoolFlag{
	Name:  "snapshot",
	Value: false,
	Usage: "Enable snapshot boost for syncing and converging",
}

var p2pTCPPortFlag = &cli.IntFlag{
	Name:  "p2p-port",
	Usage: "Port for P2P connection",
	Value: defaultTCPP2PPort,
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

var keyFileNameFlag = &cli.StringFlag{
	Name:  "keyfile",
	Usage: "The filename to the key",
	Value: "",
}

var keyPassFlag = &cli.StringFlag{
	Name:  "key-pass",
	Usage: "The password to unlock the key file",
	Value: "",
}

var p2pKeyFileFlag = &cli.StringFlag{
	Name:  "p2p-key",
	Usage: "The file path to the p2p key",
	Value: "",
}

var inMemFlag = &cli.BoolFlag{
	Name:  "in-mem",
	Usage: "Run the database of blocks, vaults in memory",
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

	p2pTCPPort := c.Int(p2pTCPPortFlag.Name)
	rpcHost := c.String(rpcHostFlag.Name)
	rpcPort := c.Int(rpcPortFlag.Name)
	rpcDisables := c.StringSlice(rpcDisableFlag.Name)
	keyPass := c.String(keyPassFlag.Name)
	keyFile := c.String(keyFileNameFlag.Name)
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

	key := keytools.ReadLocalKey(keyFile, strings.TrimSpace(keyPass))
	log.Warnf("use address: %s to receive mining rewards \n", base58.FastBase58Encoding(ngtypes.NewAddress(key)))

	var db *dbolt.DB
	db = storage.InitStorage(network, dbFolder)

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	store := ngblocks.Init(db, network)
	// then sync
	state := ngstate.InitStateFromGenesis(db, network)

	chain := blockchain.Init(db, network, store, state)
	chain.CheckHealth(network)

	localNode := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:       p2pKeyFile,
		Network:          network,
		Port:             p2pTCPPort,
		DisableDiscovery: network == ngtypes.ZERONET,
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
			DisableConnectingBootstraps: isBootstrapNode || network == ngtypes.ZERONET,
		},
	)
	pow.GoLoop()

	// when rpcPort <= 0, disable rpc server
	if rpcPort > 0 {
		jsonRPCServerConfig := jsonrpc.ServerConfig{
			Host:                 rpcHost,
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

	// notify the exit events
	select {}
}
