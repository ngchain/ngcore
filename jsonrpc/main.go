package jsonrpc

import (
	"fmt"
	"net/http"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/consensus"
)

var log = logging.Logger("rpc")

// rpcFunc is a stateless jsonrpc method handler (shared by the HTTP and WS
// transports). Subscription methods, which need the connection, are handled
// separately by the WS session dispatcher.
type rpcFunc = func(*jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage

type ServerConfig struct {
	Host                 string
	Port                 int
	DisableP2PMethods    bool
	DisableMiningMethods bool
}

// Server is a json-rpc v2 server, reachable over HTTP POST at / and over
// WebSocket at /ws (the latter additionally serves subscriptions).
type Server struct {
	*ServerConfig
	*jsonrpc2http.Server

	pow *consensus.PoWork

	// methods is the WS-callable registry: every plain method registered
	// for HTTP is also callable over the WS connection
	methods map[string]rpcFunc
	hub     *subHub
}

// reg registers a plain method for BOTH transports.
func (s *Server) reg(method string, fn rpcFunc) {
	s.RegisterJsonRpcHandleFunc(method, fn) // HTTP
	s.methods[method] = fn                  // WS
}

// NewServer will create a new Server with its methods registered, but not running.
func NewServer(pow *consensus.PoWork, config ServerConfig) *Server {
	s := &Server{
		ServerConfig: &config,
		pow:          pow,
		methods:      make(map[string]rpcFunc),
	}

	s.Server = jsonrpc2http.NewServer(jsonrpc2http.ServerConfig{
		Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler: nil,
		Logger:  log,
	})

	registerHTTPHandler(s)

	// subscriptions: fan block/log/pending-tx events out to WS sessions
	s.hub = newSubHub(s)
	s.hub.install()

	// route HTTP jsonrpc at / and the WS endpoint at /ws on the same port
	mux := http.NewServeMux()
	mux.Handle("/", s.Server.HTTPHandler)
	mux.HandleFunc("/ws", s.serveWS)
	s.Server.Server.Handler = mux

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
