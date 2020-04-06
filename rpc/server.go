package rpc

import (
	"fmt"

	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
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
	s := &Server{
		sheetManager: sheetManager,
		consensus:    consensus,
		txPool:       txPool,
		localNode:    localNode,
		Server:       nil,
	}

	s.Server = jsonrpc2http.NewServer(fmt.Sprintf("%s:%d", host, port), NewHTTPHandler(s))
	return s
}

func (s *Server) Run() {
	log.Info("rpc server running")
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
