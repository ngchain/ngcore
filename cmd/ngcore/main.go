// package main is the entry of daemon
package main

import (
	"fmt"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/rpc"
	"github.com/ngchain/ngcore/sheet"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
	"github.com/whyrusleeping/go-logging"
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"
)

var log = logging.MustGetLogger("main")

var strictModeFlag = cli.BoolFlag{
	Name:  "strict",
	Usage: "force ngcore starts from the genesis block",
}

var logFlag = cli.BoolTFlag{
	Name:  "save-log",
	Usage: "Whether save the log into file",
}

var p2pTcpPortFlag = cli.IntFlag{
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
	Usage: "start local node as a bootstrap peer",
}

var profileFlag = cli.BoolFlag{
	Name:  "profile",
	Usage: "write cpu profile to the file",
}

var keyPassFlag = cli.StringFlag{
	Name:  "key-pass",
	Usage: "The password to unlock the key file",
	Value: "",
}

var miningFlag = cli.BoolFlag{
	Name:  "mining",
	Usage: "start mining",
}

var format = logging.MustStringFormatter(
	"%{time:15:04:05.000} %{color}[%{module}] ▶ %{level}%{color:reset} %{message}",
)

// the Main
var action = func(c *cli.Context) error {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	formatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(formatter)

	isBootstrapNode := c.Bool("bootstrap")
	isMining := c.Bool("mining")
	isStrictMode := isBootstrapNode || c.Bool("strict")
	p2pTcpPort := c.Int("p2p-port")
	rpcPort := c.Int("rpc-port")
	keyPass := c.String("key-pass")
	withProfile := c.Bool("profile")

	if withProfile {
		f, err := os.Create(fmt.Sprintf("%d.cpu.profile", time.Now().Unix()))
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	logging.SetLevel(logging.INFO, "")

	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(keyPass))
	keytools.PrintPublicKey(key)

	db := storage.InitStorage()
	defer db.Close()

	chain := ngchain.NewChain(db)
	if isStrictMode && chain.GetLatestBlockHeight() == 0 {
		chain.InitWithGenesis()
		// then sync
	}
	sheetManager := sheet.NewSheetManager()
	txPool := txpool.NewTxPool()

	consensusManager := consensus.NewConsensusManager(isMining)
	consensusManager.Init(chain, sheetManager, key, txPool)

	localNode := ngp2p.NewLocalNode(p2pTcpPort, isStrictMode, sheetManager, chain, txPool)
	rpc := rpc.NewServer("127.0.0.1", rpcPort, consensusManager, localNode, sheetManager, txPool)
	go rpc.Run()

	localNode.OnSynced = func() {
		consensusManager.ResumeMining()
	}

	localNode.OnNotSynced = func() {
		consensusManager.StopMining()
	}

	if !isBootstrapNode {
		localNode.ConnectBootstrapNodes()
	}

	localNode.Init(func() {
		latestVault := chain.GetLatestVault()
		sheetManager.Init(latestVault)
		txPool.Init(latestVault, chain.MinedBlockToTxPoolCh, chain.NewVaultToTxPoolCh)
		txPool.Run()

		consensusManager.InitPoW()
	})

	// notify the exit events
	var stopSignal = make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGTERM)
	signal.Notify(stopSignal, syscall.SIGINT)
	for {
		select {
		case sign := <-stopSignal:
			log.Info("Signal received:", sign)
			return nil
		}
	}
}

func main() {
	app := cli.NewApp()

	app.Name = "NG"
	app.Usage = "NGIN Network"
	app.Version = "v0.0.1"
	app.Action = action

	flags := []cli.Flag{
		strictModeFlag, logFlag, p2pTcpPortFlag, rpcPortFlag, miningFlag,
		isBootstrapFlag, keyPassFlag, profileFlag,
	}

	app.Flags = flags
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
