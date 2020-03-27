package rpc

import (
	"fmt"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
	"github.com/whyrusleeping/go-logging"
)

var log = logging.MustGetLogger("rpc")

type Server struct {
	consensus    *consensus.Consensus
	sheetManager *ngsheet.Manager
	txPool       *txpool.TxPool

	localNode *ngp2p.LocalNode
	*jsonrpc2http.Server
}

func NewServer(host string, port int, consensus *consensus.Consensus, localNode *ngp2p.LocalNode, sheetManager *ngsheet.Manager, txPool *txpool.TxPool) *Server {
	addr := fmt.Sprintf("%s:%d", host, port)

	s := &Server{
		sheetManager: sheetManager,
		consensus:    consensus,
		txPool:       txPool,
		localNode:    localNode,
		Server:       nil,
	}

	s.Server = jsonrpc2http.NewServer(addr, NewHTTPHandler(s))
	return s
}

func (s *Server) Run() {
	s.ListenAndServe()
}
