package main

import (
	"fmt"
	"io"
	"os"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/utils"
)

func rpcCall(addr, method string, params []byte) (string, error) {
	c := jsonrpc2http.NewClient()
	msg := jsonrpc2.NewJsonRpcRequest(1, method, params)
	request, err := jsonrpc2http.NewClientRequest(addr, msg)
	if err != nil {
		return "", err
	}

	response, err := c.Do(request)
	if err != nil {
		return "", err
	}

	raw, err := io.ReadAll(response.Body)
	return string(raw), err
}

func getCliToolsCommand() *cli.Command {
	return &cli.Command{
		Name: "cli",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "a",
				Aliases: []string{"addr"},
				Usage:   "the daemon rpc server address",
				Value:   "http://localhost:52521",
			},
		},
		Description: "built-in rpc client",
		Subcommands: []*cli.Command{
			{
				Name:        "contract-update",
				Description: "diff a local contract text (wat) against the on-chain one and compose an unsigned edit tx carrying the minimal patch",
				Flags: []cli.Flag{
					&cli.Uint64Flag{
						Name:     "num",
						Usage:    "the contract account num",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "file",
						Usage:    "path to the new contract text (.wat)",
						Required: true,
					},
					&cli.Float64Flag{
						Name:  "fee",
						Usage: "tx fee in NG",
					},
				},
				Action: func(context *cli.Context) error {
					newContract, err := os.ReadFile(context.String("file"))
					if err != nil {
						return err
					}

					params, err := utils.JSON.Marshal(map[string]interface{}{
						"convener":    context.Uint64("num"),
						"fee":         context.Float64("fee"),
						"newContract": string(newContract),
					})
					if err != nil {
						return err
					}

					// the result is the unsigned tx hex: sign it with the
					// signTx method and broadcast it with sendTx
					result, err := rpcCall(context.String("a"), "genContractUpdate", params)
					if err != nil {
						return err
					}
					fmt.Println(result)

					return nil
				},
			},
		},
		Action: func(context *cli.Context) error {
			cmd := context.Args().Get(0)
			args := context.Args().Get(1)
			var params []byte
			if args != "" {
				params = []byte(args)
			}

			result, err := rpcCall(context.String("a"), cmd, params)
			if err != nil {
				return err
			}
			fmt.Println(result)

			return nil
		},
	}
}
