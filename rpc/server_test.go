package rpc

import (
	"testing"
	"time"
)

func TestNewRPCServer(t *testing.T) {
	rpc := NewRPCServer(nil, nil, nil)
	go rpc.Serve(1337)

	go func() {
		finished := time.After(2 * time.Minute)
		for {
			select {
			case <-finished:
				return
			}
		}
	}()
}
