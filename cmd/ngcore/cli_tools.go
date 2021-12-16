package main

import (
	"fmt"
	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/urfave/cli/v2"
	"io/ioutil"
)

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
		Subcommands: []*cli.Command{},
		Action: func(context *cli.Context) error {
			// create new client
			cmd := context.Args().Get(0)
			args := context.Args().Get(1)
			var params []byte
			if args == "" {
				params = nil
			} else {
				params = []byte(args)
			}
			c := jsonrpc2http.NewClient()
			msg := jsonrpc2.NewJsonRpcRequest(1, cmd, params)
			request, err := jsonrpc2http.NewClientRequest(context.String("a"), msg)
			if err != nil {
				return err
			}

			response, err := c.Do(request)
			if err != nil {
				return err
			}

			raw, _ := ioutil.ReadAll(response.Body)
			fmt.Println(string(raw))

			return nil
		},
	}
}
