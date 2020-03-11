package rpcServer

import (
	"testing"
)

func TestNewRPCServer(t *testing.T) {
	rpc := NewRPCServer(nil, nil, nil, nil)
	rpc.Serve(1337)
}
