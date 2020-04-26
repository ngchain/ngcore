package rpc_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/rpc"
)

func TestNewRPCServer(t *testing.T) {
	s := rpc.NewServer("127.0.0.1", 52521, nil, nil, nil, nil)
	go s.Run()

	go func() {
		finished := time.After(2 * time.Minute)

		for {
			<-finished
			return
		}
	}()
}
