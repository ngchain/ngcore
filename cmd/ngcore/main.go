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

	"github.com/whyrusleeping/go-logging"
	"gopkg.in/urfave/cli.v1"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/rpc"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

var log = logging.MustGetLogger("main")

var strictModeFlag = cli.BoolTFlag{
	Name:  "strict",
	Usage: "Enable forcing ngcore starts from the genesis block",
}

var logFlag = cli.StringFlag{
	Name:  "log-file",
	Usage: "Enable save the log into the file",
}

var p2pTCPPortFlag = cli.IntFlag{
	Name:  "p2p-port",
	Usage: "Port for P2P connection",
	Value: 52520,
}

var rpcPortFlag = cli.IntFlag{
	Name:  "rpc-port",
	Usage: "Port for RPC",
	Value: 52521,
}

var isBootstrapFlag = cli.BoolFlag{
	Name:  "bootstrap",
	Usage: "Enable starting local node as a bootstrap peer",
}

var profileFlag = cli.BoolFlag{
	Name:  "profile",
	Usage: "Enable writing cpu profile to the file",
}

var keyPassFlag = cli.StringFlag{
	Name:  "key-pass",
	Usage: "The password to unlock the key file",
	Value: "",
}

var miningFlag = cli.IntFlag{
	Name:  "mining",
	Usage: "The worker number on mining. Mining starts when value is not negative. And when value equals to 0, use all cpu cores",
	Value: -1,
}

var colorFlag = cli.BoolFlag{
	Name:  "color",
	Usage: "Enable displaying log with color",
}

var logLevelFlag = cli.StringFlag{
	Name:  "log-level",
	Value: "INFO",
	Usage: "Enable displaying logs which are equal or higher to the level. Values can be ERROR, WARNING, NOTICE, INFO or DEBUG",
}

// the Main
var action = func(c *cli.Context) error {
	var format string
	if c.Bool("color") {
		format = "%{time:15:04:05.000} %{color}[%{module}] ▶ %{level}%{color:reset} %{message}"
	} else {
		format = "%{time:15:04:05.000} [%{module}] ▶ %{level} %{message}"
	}

	strFormatter := logging.MustStringFormatter(format)
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, strFormatter)
	logging.SetBackend(backendFormatter)

	var logLevel logging.Level
	switch strings.ToUpper(c.String("log-level")) {
	case "ERROR":
		logLevel = logging.ERROR
	case "WARNING":
		logLevel = logging.WARNING
	case "NOTICE":
		logLevel = logging.NOTICE
	case "INFO":
		logLevel = logging.INFO
	case "DEBUG":
		logLevel = logging.DEBUG
	default:
		panic("unknown log level:" + c.String("log-level"))
	}

	logging.SetLevel(logLevel, "")

	isBootstrapNode := c.Bool("bootstrap")
	isMining := c.Int("mining") >= 0
	isStrictMode := isBootstrapNode || c.BoolT("strict")
	p2pTCPPort := c.Int("p2p-port")
	rpcPort := c.Int("rpc-port")
	keyPass := c.String("key-pass")
	withProfile := c.Bool("profile")

	if withProfile {
		f, err := os.Create(fmt.Sprintf("%d.cpu.profile", time.Now().Unix()))
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}

	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(keyPass))
	keytools.PrintPublicKey(key)

	db := storage.InitStorage()
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

	latestVault := chain.GetLatestVault()
	blocks, err := chain.GetBlocksOnVaultHeight(latestVault.GetHeight())
	if err != nil {
		panic(err)
	}
	sheetManager.Init(latestVault, blocks...)
	txPool.Init(latestVault, chain.MinedBlockToTxPoolCh, chain.NewVaultToTxPoolCh)
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
	app.Version = "v0.0.2"
	app.Action = action

	flags := []cli.Flag{
		strictModeFlag, logFlag, p2pTCPPortFlag, rpcPortFlag, miningFlag,
		isBootstrapFlag, keyPassFlag, profileFlag,
		colorFlag, logLevelFlag,
	}

	app.Flags = flags
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
