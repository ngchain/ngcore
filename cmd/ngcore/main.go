// package main is the entry of daemon
package main

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("main")

func main() {
	app := cli.NewApp()

	app.Name = "ngcore"
	app.Usage = usage
	app.Description = description
	app.Version = version
	app.Action = action

	app.Flags = []cli.Flag{
		strictModeFlag, logFlag, p2pTCPPortFlag, apiPortFlag, miningFlag,
		isBootstrapFlag, keyPassFlag, profileFlag,
		logLevelFlag,
		inMemFlag,
	}

	app.Commands = []*cli.Command{
		getKeyToolsCommand(), genesistoolsCommand,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
