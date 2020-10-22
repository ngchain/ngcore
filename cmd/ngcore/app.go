package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/storage"

	logging "github.com/ipfs/go-log/v2"
	"github.com/mr-tron/base58"

	"github.com/dgraph-io/badger/v2"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

var strictModeFlag = &cli.BoolFlag{
	Name:  "strict",
	Value: true,
	Usage: "Enable forcing ngcore starts from the genesis block",
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

var isBootstrapFlag = &cli.BoolFlag{
	Name:  "bootstrap",
	Usage: "Enable starting local node as a bootstrap peer",
}

var testNetFlag = &cli.BoolFlag{
	Name:  "testnet",
	Usage: "Run node on the test network",
}

var regTestNetFlag = &cli.BoolFlag{
	Name:  "reg-testnet",
	Usage: "Run node on the regression test network",
}

var profileFlag = &cli.BoolFlag{
	Name:  "profile",
	Usage: "Enable writing cpu profile to the file",
}

var keyFileFlag = &cli.StringFlag{
	Name:  "key-file",
	Usage: "The filename to the key",
	Value: "",
}

var keyPassFlag = &cli.StringFlag{
	Name:  "key-pass",
	Usage: "The password to unlock the key file",
	Value: "",
}

var miningFlag = &cli.IntFlag{
	Name: "mining",
	Usage: "The worker number on mining. Mining starts when value is not negative. " +
		"And when value equals to 0, use all cpu cores",
	Value: defaultMiningThread,
}

var logFileFlag = &cli.StringFlag{
	Name:  "log-file",
	Value: defaultLogFile,
	Usage: "Enable save the log into the file",
}

var logLevelFlag = &cli.StringFlag{
	Name:  "log-level",
	Value: defaultLogLevel,
	Usage: "Enable displaying logs which are equal or higher to the level. " +
		"Values can be ERROR, WARN, INFO or DEBUG",
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

var action = func(c *cli.Context) error {
	logLevel, err := logging.LevelFromString(c.String("log-level"))
	if err != nil {
		panic(err)
	}

	logConf := logging.Config{
		Level:  logLevel,
		Stdout: c.String("log-file") == "",
		File:   c.String("log-file"),
	}
	if len(logConf.File) > 0 {
		fmt.Println("logging to", logConf.File)
	}

	logging.SetupLogging(logConf)

	isBootstrapNode := c.Bool("bootstrap")
	mining := c.Int("mining")
	if mining == 0 {
		mining = runtime.NumCPU()
	}

	isStrictMode := isBootstrapNode || c.Bool(strictModeFlag.Name)
	p2pTCPPort := c.Int(p2pTCPPortFlag.Name)
	rpcHost := c.String(rpcHostFlag.Name)
	rpcPort := c.Int(rpcPortFlag.Name)
	keyPass := c.String(keyPassFlag.Name)
	keyFile := c.String(keyFileFlag.Name)
	withProfile := c.Bool(profileFlag.Name)
	inMem := c.Bool(inMemFlag.Name)
	dbFolder := c.String(dbFolderFlag.Name)

	var network = ngtypes.NetworkType_TESTNET
	if c.Bool(testNetFlag.Name) {
		network = ngtypes.NetworkType_TESTNET
	}

	if c.Bool(regTestNetFlag.Name) {
		network = ngtypes.NetworkType_ZERONET // use zero net as the regression test network
	}

	if withProfile {
		var f *os.File

		f, err = os.Create(fmt.Sprintf("%d.cpu.profile", time.Now().Unix()))
		if err != nil {
			panic(err)
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			panic(err)
		}

		defer pprof.StopCPUProfile()
	}

	key := keytools.ReadLocalKey(keyFile, strings.TrimSpace(keyPass))
	fmt.Printf("Use address: %s to receive mining rewards \n", base58.FastBase58Encoding(ngtypes.NewAddress(key)))

	var db *badger.DB
	if inMem {
		db = storage.InitMemStorage()
	} else {
		db = storage.InitStorage(dbFolder)
	}

	defer func() {
		err = db.Close()
		if err != nil {
			panic(err)
		}
	}()

	var store *ngblocks.BlockStore
	if isStrictMode {
		store = ngblocks.Init(db, network)
		// then sync
	} else {
		// TODO: use the new origin block to initialize the ngblocks
		store = ngblocks.Init(db, network)
	}
	state := ngstate.InitStateFromGenesis(db, network)

	chain := ngchain.Init(db, network, store, state)

	ngp2p.InitLocalNode(network, p2pTCPPort, chain)
	ngp2p.GoServe()

	pool := ngpool.Init(db, chain)

	pow := consensus.InitPoWConsensus(
		db,
		chain,
		pool,
		state,
		consensus.PoWorkConfig{
			Network:                     network,
			DisableConnectingBootstraps: isBootstrapNode || network == ngtypes.NetworkType_ZERONET,
			MiningThread:                mining,
			PrivateKey:                  key,
		})
	pow.GoLoop()

	rpc := jsonrpc.NewServer(rpcHost, rpcPort, pow)
	go rpc.Serve()

	// notify the exit events
	select {}
}
