// package main is the entry of daemon
package main

import (
	"fmt"
	"github.com/ngchain/ngcore/chain"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
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
	"sync"
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
	"%{module} %{color}%{time:15:04:05.000} ▶ %{level:.4s}%{color:reset} %{message}",
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

	chain := chain.NewChain(db)
	if isStrictMode && chain.GetLatestBlockHeight() == 0 {
		chain.InitWithGenesis()
		// then sync
	}
	sheetManager := sheet.NewSheetManager()
	txPool := txpool.NewTxPool()

	consensusManager := consensus.NewConsensusManager(isMining)
	consensusManager.Init(chain, sheetManager, key, txPool)

	rpc := rpc.NewRPCServer(sheetManager, chain, txPool)
	go rpc.Serve(rpcPort)

	isSynced := false

	localNode := ngp2p.NewLocalNode(p2pTcpPort, isStrictMode, sheetManager, chain, txPool)
	if !isBootstrapNode {
		localNode.ConnectBootstrapNodes()
	}

	init := new(sync.Once)
	go func() {
		for {
			if status := localNode.IsSynced(); status && status != isSynced {
				init.Do(func() {

					if chain.GetLatestBlockHeight() == 0 {
						chain.InitWithGenesis()
					}
					latestVault := chain.GetLatestVault()
					sheetManager.Init(latestVault)
					txPool.Init(latestVault, chain.NewMinedBlockEvent, chain.NewVaultEvent)
					txPool.Run()
					log.Info("Start PoW consensus")

					foundCh := make(chan *ngtypes.Block)
					consensusManager.InitPoW(foundCh)

				})
				log.Info("localnode is synced with network")
				if isMining {
					consensusManager.ResumeMining()
				}
				isSynced = true
			} else if !status && status != isSynced {
				log.Info("localnode is not synced with network, syncing...")
				if isMining {
					consensusManager.StopMining()
				}
				isSynced = false
			}

			time.Sleep(ngtypes.TargetTime)
		}
	}()

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
