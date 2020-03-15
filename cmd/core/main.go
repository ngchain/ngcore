// package main is the entry of daemon
package main

import (
	"crypto/elliptic"
	"fmt"
	"github.com/mr-tron/base58"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/consensus"
	"github.com/ngin-network/ngcore/keyManager"
	"github.com/ngin-network/ngcore/ngp2p"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/rpcServer"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/storage"
	"github.com/ngin-network/ngcore/txpool"
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

// the Main
var action = func(c *cli.Context) error {
	isBootstrap := c.Bool("bootstrap")
	isStrictMode := isBootstrap || c.Bool("strict")
	p2pTcpPort := c.Int("p2p-port")
	rpcPort := c.Int("rpc-port")
	isMining := c.Bool("mining")
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

	keyManager := keyManager.NewKeyManager("ngcore.key", strings.TrimSpace(keyPass))
	key := keyManager.ReadLocalKey()

	db := storage.InitStorage()

	chain := chain.NewChain(db)
	if isStrictMode {
		chain.InitWithGenesis()
	}
	sheetManager := sheetManager.NewSheetManager()
	txPool := txpool.NewTxPool()

	consensusManager := consensus.NewConsensusManager(isMining)
	consensusManager.Init(chain, sheetManager, key, txPool)

	publicKey := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	fmt.Printf("LocalNode PublicKey is %v\n", base58.FastBase58Encoding(publicKey[:]))

	rpc := rpcServer.NewRPCServer(sheetManager, chain, txPool)
	go rpc.Serve(rpcPort)
	localNode := ngp2p.NewP2PNode(p2pTcpPort, isBootstrap, sheetManager, chain, txPool)

	isSynced := false
	switchCh := make(chan bool)

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
					txPool.Init(latestVault)
					log.Info("Start PoW consensus")
					go consensusManager.InitPoW()
				})
				log.Info("localnode is synced with network")
				switchCh <- true
				isSynced = true
			} else if !status && status != isSynced {
				log.Info("localnode is not synced with network, syncing...")
				switchCh <- false
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
