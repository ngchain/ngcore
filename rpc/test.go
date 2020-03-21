package rpc

import "net/http"

type Test struct{}

type PingReply struct {
	Message string `json:"message"`
}

func NewTestModule() *Test {
	return &Test{}
}

func (t *Test) Ping(r *http.Request, args *struct{}, reply *PingReply) error {
	reply.Message = "Pong"

	return nil
}
