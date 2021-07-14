package main

import (
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/keytools"
)

func getKeyToolsCommand() *cli.Command {
	filenameFlag := &cli.StringFlag{
		Name:    "filename",
		Aliases: []string{"f"},
		Value:   "",
		Usage:   "when empty, keyfile will be written into " + keytools.GetDefaultFile(),
	}

	passwordFlag := &cli.StringFlag{
		Name:    "password",
		Aliases: []string{"p"},
	}

	privateKeyFlag := &cli.StringFlag{
		Name:    "privateKey",
		Aliases: []string{"pk"},
	}

	newKeyCommand := &cli.Command{
		Name:  "new",
		Usage: "create a new key pair only",
		Flags: nil,
		Action: func(context *cli.Context) error {
			key := keytools.NewLocalKey()
			keytools.PrintKeysAndAddress(key)
			return nil
		},
	}

	createKeyCommand := &cli.Command{
		Name:  "create",
		Usage: "create a new key pair and save into the key file",
		Flags: []cli.Flag{filenameFlag, passwordFlag},
		Action: func(ctx *cli.Context) error {
			key := keytools.CreateLocalKey(strings.TrimSpace(ctx.String("filename")), strings.TrimSpace(ctx.String("password")))
			keytools.PrintKeysAndAddress(key)
			return nil
		},
	}

	recoverKeyCommand := &cli.Command{
		Name:  "recover",
		Usage: "recover the keyfile from a privateKey string",
		Flags: []cli.Flag{filenameFlag, passwordFlag, privateKeyFlag},
		Action: func(ctx *cli.Context) error {
			key := keytools.RecoverLocalKey(
				strings.TrimSpace(ctx.String("filename")),
				strings.TrimSpace(ctx.String("password")),
				strings.TrimSpace(ctx.String("privateKey")),
			)
			keytools.PrintKeysAndAddress(key)
			return nil
		},
	}

	parseKeyCommand := &cli.Command{
		Name:  "parse",
		Usage: "parse keys from a key file",
		Flags: []cli.Flag{filenameFlag, passwordFlag},
		Action: func(ctx *cli.Context) error {
			key := keytools.ReadLocalKey(strings.TrimSpace(ctx.String("filename")), strings.TrimSpace(ctx.String("password")))
			keytools.PrintKeysAndAddress(key)
			return nil
		},
	}

	return &cli.Command{
		Name:        "keytools",
		Description: "a little built-in key tools for the key file",
		Subcommands: []*cli.Command{newKeyCommand, parseKeyCommand, createKeyCommand, recoverKeyCommand},
	}
}
