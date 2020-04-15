package main

import (
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/keytools"
)

func getKeyToolsCommand() *cli.Command {
	var filenameFlag = &cli.StringFlag{
		Name:    "filename",
		Aliases: []string{"f"},
		Value:   "ngcore.key",
	}

	var passwordFlag = &cli.StringFlag{
		Name:    "password",
		Aliases: []string{"p"},
	}

	var newKeyCommand = &cli.Command{
		Name:  "new",
		Flags: []cli.Flag{filenameFlag, passwordFlag},
		Action: func(context *cli.Context) error {
			newKeyPair(context.String("filename"), context.String("password"))
			return nil
		},
	}

	var parseKeyCommand = &cli.Command{
		Name:  "parse",
		Flags: []cli.Flag{filenameFlag, passwordFlag},
		Action: func(context *cli.Context) error {
			parseKeyFile(context.String("filename"), context.String("password"))
			return nil
		},
	}

	return &cli.Command{
		Name:        "keytools",
		Description: "a little built-in key manager for the key file",
		Subcommands: []*cli.Command{newKeyCommand, parseKeyCommand},
	}
}

func newKeyPair(filename string, pass string) {
	key := keytools.CreateLocalKey(strings.TrimSpace(filename), strings.TrimSpace(pass))
	keytools.PrintKeyPair(key)
}

func parseKeyFile(filename string, pass string) {
	key := keytools.ReadLocalKey(strings.TrimSpace(filename), strings.TrimSpace(pass))
	keytools.PrintKeyPair(key)
}
