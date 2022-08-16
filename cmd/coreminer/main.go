package main

import (
	"os"

	logging "github.com/ngchain/zap-log"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("miner")

func main() {
	app := cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Description = description
	app.Version = Version
	app.Action = mining
	app.Flags = []cli.Flag{
		coreAddrFlag, corePortFlag,
		keyFileFlag, keyPassFlag,
		networkFlag,
	}

	app.Commands = []*cli.Command{}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	select {}
	// os.Exit(0)
}
