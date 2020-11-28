package main

import "github.com/urfave/cli/v2"

func getCliToolsCommand() *cli.Command {
	return &cli.Command{
		Name:        "cli",
		Flags:       nil,
		Description: "built-in rpc client",
		Subcommands: []*cli.Command{},
	}
}
