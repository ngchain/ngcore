package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/dgraph-io/badger/v2"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var strictModeFlag = &cli.BoolFlag{
	Name:  "strict",
	Value: true,
	Usage: "Enable forcing ngcore starts from the genesis block",
}

var logFlag = &cli.StringFlag{
	Name:  "log-file",
	Value: "",
	Usage: "Enable save the log into the file",
}

var p2pTCPPortFlag = &cli.IntFlag{
	Name:  "p2p-port",
	Usage: "Port for P2P connection",
	Value: defaultTCPP2PPort,
}

var apiPortFlag = &cli.IntFlag{
	Name:  "api-port",
	Usage: "Port for API",
	Value: defaultAPIPort,
}

var isBootstrapFlag = &cli.BoolFlag{
	Name:  "bootstrap",
	Usage: "Enable starting local node as a bootstrap peer",
}

var profileFlag = &cli.BoolFlag{
	Name:  "profile",
	Usage: "Enable writing cpu profile to the file",
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
	Value: -1,
}

var logLevelFlag = &cli.StringFlag{
	Name:  "log-level",
	Value: "INFO",
	Usage: "Enable displaying logs which are equal or higher to the level. " +
		"Values can be ERROR, WARNING, NOTICE, INFO or DEBUG",
}

var inMemFlag = &cli.BoolFlag{
	Name:  "in-mem",
	Usage: "Run the database of blocks, vaults in memory",
}

var action = func(c *cli.Context) error {
	logLevel, err := logging.LevelFromString(c.String("log-level"))
	if err != nil {
		panic(err)
	}

	logging.SetAllLoggers(logLevel)

	isBootstrapNode := c.Bool("bootstrap")
	mining := c.Int("mining")
	if mining == 0 {
		mining = runtime.NumCPU()
	}

	isStrictMode := isBootstrapNode || c.Bool("strict")
	p2pTCPPort := c.Int("p2p-port")
	apiPort := c.Int("api-port")
	keyPass := c.String("key-pass")
	withProfile := c.Bool("profile")
	inMem := c.Bool("in-mem")

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

	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(keyPass))
	keytools.PrintPublicKey(key)

	var db *badger.DB
	if inMem {
		db = storage.InitMemStorage()
	} else {
		db = storage.InitStorage()
	}
	defer func() {
		err = db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := storage.NewChain(db)
	if isStrictMode && chain.GetLatestBlockHeight() == 0 {
		chain.InitWithGenesis()
		// then sync
	}

	_ = ngp2p.NewLocalNode(p2pTCPPort)

	_, err = ngstate.NewStateFromSheet(ngtypes.GenesisSheet)
	if err != nil {
		panic(err)
	}

	pow := consensus.NewPoWConsensus(mining, key, isBootstrapNode)
	pow.GoLoop()

	rpc := jsonrpc.NewServer("127.0.0.1", apiPort)
	rpc.GoServe()

	// notify the exit events
	var stopSignal = make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGTERM)
	signal.Notify(stopSignal, syscall.SIGINT)
	for {
		sign := <-stopSignal
		log.Info("Signal received:", sign)
		return nil
	}
}
