package rpcServer

import (
	"fmt"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/txpool"
	"github.com/whyrusleeping/go-logging"
	"net/http"
)

var log = logging.MustGetLogger("rpc")

type RpcServer struct {
	sheetManager *sheetManager.SheetManager
	server       *rpc.Server
}

func NewRPCServer(sheetManager *sheetManager.SheetManager, blockChain *chain.BlockChain, vaultChain *chain.VaultChain, txPool *txpool.TxPool) *RpcServer {
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "*/*")

	err := s.RegisterService(NewSheetModule(sheetManager), "")
	if err != nil {
		log.Panic(err)
	}

	err = s.RegisterService(NewBlockModule(blockChain), "")
	if err != nil {
		log.Panic(err)
	}

	err = s.RegisterService(NewVaultModule(vaultChain), "")
	if err != nil {
		log.Panic(err)
	}

	err = s.RegisterService(NewTxModule(txPool), "")
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
