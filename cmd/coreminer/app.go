package main

import (
	"github.com/urfave/cli/v2"
)

var coreAddrFlag = &cli.StringFlag{
	Name:    "addr",
	Aliases: []string{"a"},
	Usage:   "ngcore address for JSON RPC",
	Value:   defaultRPCHost,
}

var corePortFlag = &cli.IntFlag{
	Name:    "port",
	Aliases: []string{"p"},
	Usage:   "ngcore address for JSON RPC",
	Value:   defaultRPCPort,
}
