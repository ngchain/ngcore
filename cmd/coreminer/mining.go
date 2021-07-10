package main

import (
	"net"
	"strconv"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/urfave/cli/v2"
)

var mining cli.ActionFunc = func(context *cli.Context) error {
	client := jsonrpc2http.NewClient()

	baseURL := "http://" + net.JoinHostPort(context.String(coreAddrFlag.Name), strconv.Itoa(context.Int(corePortFlag.Name)))
	request, err := jsonrpc2http.NewClientRequest(baseURL, &jsonrpc2.JsonRpcMessage{
		Method: "",
		Params: nil,
		Result: nil,
		Error:  nil,
		ID:     nil,
	})
	if err != nil {
		return err
	}

	_, err = client.Do(request)
	if err != nil {
		return err
	}

	return nil
}
