package jsonrpc_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/jsonrpc"
)

// TODO: add tests for each method
func TestNewRPCServer(t *testing.T) {
	rpc := jsonrpc.NewServer("", 52521, nil)
	go rpc.Serve()

	go func() {
		finished := time.After(2 * time.Minute)
		for {
			<-finished
			return
		}
	}()
}
