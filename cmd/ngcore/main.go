// package main is the entry of daemon
package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Description = description
	app.Version = Version
	app.Action = action
	app.Flags = []cli.Flag{
		nonStrictModeFlag, snapshotModeFlag, minerExtraFlag,
		p2pTCPPortFlag, p2pKeyFileFlag,
		rpcHostFlag, rpcPortFlag, rpcDisableFlag,
		isBootstrapFlag, profileFlag,

		inMemFlag, dbFolderFlag,

		testNetFlag, zeroNetFlag,
	}

	app.Commands = []*cli.Command{
		getKeyToolsCommand(),
		getCliToolsCommand(),
		getContractTestCommand(),
		getDevnetCommand(),
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
