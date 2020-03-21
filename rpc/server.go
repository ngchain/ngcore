package rpc

import (
	"fmt"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	"github.com/ngchain/ngcore/chain"
	"github.com/ngchain/ngcore/sheet"
	"github.com/ngchain/ngcore/txpool"
	"github.com/whyrusleeping/go-logging"
	"net/http"
)

var log = logging.MustGetLogger("rpc")

type RpcServer struct {
	sheetManager *sheet.Manager
	server       *rpc.Server
}

func NewRPCServer(sheetManager *sheet.Manager, chain *chain.Chain, txPool *txpool.TxPool) *RpcServer {
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "*/*")

	err := s.RegisterService(NewChainModule(chain), "")
	if err != nil {
		log.Panic(err)
	}

	err = s.RegisterService(NewTxModule(txPool, sheetManager), "")
	if err != nil {
		log.Panic(err)
	}

	err = s.RegisterService(new(Test), "")
	if err != nil {
		log.Panic(err)
	}

	return &RpcServer{
		server:       s,
		sheetManager: sheetManager,
	}
}

func (s *RpcServer) Serve(port int) {
	http.Handle("/", s.server)
	err := http.ListenAndServe(
		fmt.Sprintf("127.0.0.1:%d", port),
		nil,
	)
	if err != nil {
		log.Panic(err)
	}
}
