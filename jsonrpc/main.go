package jsonrpc

import (
	"fmt"

	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/consensus"
)

var log = logging.Logger("rpc")

type ServerConfig struct {
	Host                 string
	Port                 int
	DisableP2PMethods    bool
	DisableMiningMethods bool
}

// Server is a json-rpc v2 server.
type Server struct {
	*ServerConfig
	*jsonrpc2http.Server

	pow *consensus.PoWork
}

// NewServer will create a new Server, with registered *jsonrpc2http.HTTPHandler. But not running.
func NewServer(pow *consensus.PoWork, config ServerConfig) *Server {
	s := &Server{
		ServerConfig: &config,
		Server:       nil,

		pow: pow,
	}

	s.Server = jsonrpc2http.NewServer(jsonrpc2http.ServerConfig{
		Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler: nil,
		Logger:  log,
	})

	registerHTTPHandler(s)

	return s
}

// Serve will make the server running.
func (s *Server) Serve() {
	log.Warnf("JSON RPC listening on: %s \n", s.Addr)
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
