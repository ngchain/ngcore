package rpc

import (
	"testing"
	"time"
)

func TestNewRPCServer(t *testing.T) {
	rpc := NewServer("127.0.0.1", 52521, nil, nil, nil, nil)
	go rpc.Run()

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
