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
	"github.com/ngin-network/ngcore/rpcServer"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/storage"
	"github.com/ngin-network/ngcore/txpool"
	"github.com/whyrusleeping/go-logging"
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var log = logging.MustGetLogger("main")

var daemonFlag = cli.BoolFlag{
	Name:  "daemon, d",
	Usage: "Start ngcore as a Daemon (background)",
}

var logFlag = cli.BoolTFlag{
	Name:  "save-log",
	Usage: "Whether save the log into file",
}

var p2pTcpPortFlag = cli.IntFlag{
	Name:  "p2p-tcp-port",
	Usage: "Port for P2P connection",
	Value: 52520,
}

var p2pWsPortFlag = cli.IntFlag{
	Name:  "p2p-ws-port",
	Usage: "Port for P2P connection",
	Value: 52530,
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
	var p2pTcpPort, p2pWsPort, rpcPort int
	var keyPass string
	var mining, isBootstrap, withProfile bool

	fmt.Println("Loading blockchain")
	p2pTcpPort = c.Int("p2p-tcp-port")
	p2pWsPort = c.Int("p2p-ws-port")
	rpcPort = c.Int("rpc-port")
	isBootstrap = c.Bool("bootstrap")
	mining = c.Bool("mining")
	keyPass = c.String("key-pass")
	withProfile = c.Bool("profile")

	logging.SetLevel(logging.INFO, "")

	localMgr := keyManager.NewKeyManager("ngcore.key", strings.TrimSpace(keyPass))
	localKey := localMgr.ReadLocalKey()

	db := storage.InitStorage()

	blockChain := chain.NewBlockChain(db)
	vaultChain := chain.NewVaultChain(db)

	sheetManager := sheetManager.NewSheetManager(vaultChain.GetLatestVault())

	txPool := txpool.NewTxPool(vaultChain.GetLatestVault())

	consensusManager := consensus.InitConsensusManager(blockChain, vaultChain, sheetManager, localKey, txPool)

	publicKey := elliptic.Marshal(elliptic.P256(), localKey.PublicKey.X, localKey.PublicKey.Y)
	fmt.Printf("Node PublicKey is %v\n", base58.FastBase58Encoding(publicKey[:]))

	fmt.Println(p2pTcpPort, p2pWsPort, rpcPort, mining, isBootstrap, withProfile) // placeholder

	s := ngp2p.NewP2PServer()
	s.Serve(p2pTcpPort, isBootstrap, sheetManager, blockChain, vaultChain, txPool)

	go consensusManager.PoW(mining)

	rpc := rpcServer.NewRPCServer(sheetManager, blockChain, vaultChain, txPool)
	go rpc.Serve(rpcPort)

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
		daemonFlag, logFlag, p2pTcpPortFlag, p2pWsPortFlag, rpcPortFlag, miningFlag,
		isBootstrapFlag, keyPassFlag, profileFlag,
	}

	app.Flags = flags
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
