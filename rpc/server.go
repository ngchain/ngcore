package rpc

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
)

var log = logging.Logger("rpc")

// Server is a json-rpc v2 server
type Server struct {
	consensus    *consensus.Consensus
	sheetManager *ngsheet.SheetManager
	txPool       *txpool.TxPool

	localNode *ngp2p.LocalNode
	*jsonrpc2http.Server
}

// NewServer will create a new Server, with registered *jsonrpc2http.HTTPHandler. But not running
func NewServer(host string, port int, consensus *consensus.Consensus, localNode *ngp2p.LocalNode, sheetManager *ngsheet.SheetManager, txPool *txpool.TxPool) *Server {
	s := &Server{
		sheetManager: sheetManager,
		consensus:    consensus,
		txPool:       txPool,
		localNode:    localNode,
		Server:       nil,
	}

	s.Server = jsonrpc2http.NewServer(fmt.Sprintf("%s:%d", host, port), newHTTPHandler(s))
	return s
}

// Run will make the server running
func (s *Server) Run() {
	log.Info("rpc server running")
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
