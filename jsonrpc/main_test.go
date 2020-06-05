package jsonrpc_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/jsonrpc"
)

// TODO: add tests for each method
func TestNewRPCServer(t *testing.T) {
	rpc := jsonrpc.NewServer("127.0.0.1", 52521)
	go rpc.GoServe()

	go func() {
		finished := time.After(2 * time.Minute)
		for {
			<-finished
			return
		}
	}()
}
