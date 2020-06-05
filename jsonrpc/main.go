package jsonrpc

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
)

var log = logging.Logger("rpc")

// Server is a json-rpc v2 server
type Server struct {
	*jsonrpc2http.Server
}

// NewServer will create a new Server, with registered *jsonrpc2http.HTTPHandler. But not running
func NewServer(host string, port int) *Server {
	s := &Server{
		Server: nil,
	}

	s.Server = jsonrpc2http.NewServer(fmt.Sprintf("%s:%d", host, port), newHTTPHandler(s))
	return s
}

// GoServe will make the server running
func (s *Server) GoServe() {
	log.Info("rpc server running")
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
