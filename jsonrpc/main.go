package jsonrpc

import (
	"fmt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
	"github.com/ngchain/ngcore/consensus"
)

var log = logging.Logger("rpc")

// Server is a json-rpc v2 server
type Server struct {
	*jsonrpc2http.Server

	pow *consensus.PoWork
}

// NewServer will create a new Server, with registered *jsonrpc2http.HTTPHandler. But not running
func NewServer(host string, port int, pow *consensus.PoWork) *Server {
	s := &Server{
		Server: nil,

		pow: pow,
	}

	s.Server = jsonrpc2http.NewServer(fmt.Sprintf("%s:%d", host, port), newHTTPHandler(s))
	return s
}

// Serve will make the server running
func (s *Server) Serve() {
	log.Warnf("JSON RPC listening on: %s \n", s.Addr)
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
