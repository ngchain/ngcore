// package main is the entry of daemon
package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/rpc"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

var log = logging.Logger("main")

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
	Value: 52520,
}

var rpcPortFlag = &cli.IntFlag{
	Name:  "rpc-port",
	Usage: "Port for RPC",
	Value: 52521,
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
	Name:  "mining",
	Usage: "The worker number on mining. Mining starts when value is not negative. And when value equals to 0, use all cpu cores",
	Value: -1,
}

var logLevelFlag = &cli.StringFlag{
	Name:  "log-level",
	Value: "INFO",
	Usage: "Enable displaying logs which are equal or higher to the level. Values can be ERROR, WARNING, NOTICE, INFO or DEBUG",
}

var inMemFlag = &cli.BoolFlag{
	Name:  "in-mem",
	Usage: "Run the database of blocks, vaults in memory",
}

// the Main
var action = func(c *cli.Context) error {
	logLevel, err := logging.LevelFromString(c.String("log-level"))
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(logLevel)

	isBootstrapNode := c.Bool("bootstrap")
	isMining := c.Int("mining") >= 0
	isStrictMode := isBootstrapNode || c.Bool("strict")
	p2pTCPPort := c.Int("p2p-port")
	rpcPort := c.Int("rpc-port")
	keyPass := c.String("key-pass")
	withProfile := c.Bool("profile")
	inMem := c.Bool("in-mem")

	if withProfile {
		f, err := os.Create(fmt.Sprintf("%d.cpu.profile", time.Now().Unix()))
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
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := ngchain.NewChain(db)
	if isStrictMode && chain.GetLatestBlockHeight() == 0 {
		chain.InitWithGenesis()
		// then sync
	}
	sheetManager := ngsheet.NewSheetManager()
	txPool := txpool.NewTxPool(sheetManager)

	consensus := consensus.NewConsensus(isMining)
	consensus.Init(chain, sheetManager, key, txPool)

	localNode := ngp2p.NewLocalNode(consensus, p2pTCPPort, isStrictMode, isBootstrapNode)

	rpc := rpc.NewServer("127.0.0.1", rpcPort, consensus, localNode, sheetManager, txPool)
	go rpc.Run()

	latestBlock := chain.GetLatestBlock()
	sheetManager.Init(latestBlock)
	txPool.Init(chain.MinedBlockToTxPoolCh)
	txPool.Run()

	initOnce := &sync.Once{}
	localNode.OnSynced = func() {
		initOnce.Do(func() {
			consensus.InitPoW(c.Int("mining"))
		})

		consensus.Resume()
	}

	localNode.OnNotSynced = func() {
		consensus.Stop()
	}

	localNode.Init()

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

func main() {
	app := cli.NewApp()

	app.Name = "ngcore"
	app.Usage = "Brand-new golang daemon implement of Ngin Network Node"
	app.Description = `NGIN is a radically updating brand-new blockchain network, which is not a fork of ethereum or any other chain.`
	app.Version = "v0.0.7"
	app.Action = action

	app.Flags = []cli.Flag{
		strictModeFlag, logFlag, p2pTCPPortFlag, rpcPortFlag, miningFlag,
		isBootstrapFlag, keyPassFlag, profileFlag,
		logLevelFlag,
		inMemFlag,
	}

	// TODO integrate tools into subcommands
	app.Commands = []*cli.Command{
		GetKeyToolsFlag(), genesistoolsCommand,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
