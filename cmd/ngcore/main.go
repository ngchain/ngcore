// package main is the entry of daemon
package main

import (
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()

	app.Name = "ngcore"
	app.Usage = usage
	app.Description = description
	app.Version = version
	app.Action = action

	app.Flags = []cli.Flag{
		nonStrictModeFlag, snapshotModeFlag,
		p2pTCPPortFlag, p2pKeyFileFlag,
		rpcHostFlag, rpcPortFlag, rpcDisableFlag,
		isBootstrapFlag, profileFlag,
		keyFileNameFlag, keyPassFlag,

		inMemFlag, dbFolderFlag,

		testNetFlag, zeroNetFlag,
	}

	app.Commands = []*cli.Command{
		getKeyToolsCommand(), getGenesisToolsCommand(),
	}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	os.Exit(0)
}
