// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/restapi/operations"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
	"github.com/ngchain/ngcore/utils"
)

//go:generate swagger generate server --target ../../ngcore --name  --spec ../swagger-ui/swagger.json

func configureFlags(api *operations.EmptyAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *operations.EmptyAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.GetHandler = operations.GetHandlerFunc(func(params operations.GetParams) middleware.Responder {
		return operations.NewGetOK().WithPayload("hello world")
	})
	api.GetAccountAllHandler = operations.GetAccountAllHandlerFunc(
		func(params operations.GetAccountAllParams) middleware.Responder {
			return middleware.NotImplemented("not implemented")
		},
	)

	api.GetAccountNumHandler = operations.GetAccountNumHandlerFunc(func(params operations.GetAccountNumParams) middleware.Responder {
		return middleware.NotImplemented("operation operations.GetAccountAtNum has not yet been implemented")
	})

	api.GetAccountNumBalanceHandler = operations.GetAccountNumBalanceHandlerFunc(func(params operations.GetAccountNumBalanceParams) middleware.Responder {
		return middleware.NotImplemented("operation operations.GetAccountAtNumBalance has not yet been implemented")
	})

	api.GetAccountMyHandler = operations.GetAccountMyHandlerFunc(func(params operations.GetAccountMyParams) middleware.Responder {
		key := utils.PublicKey2Bytes(*consensus.GetPoWConsensus().PrivateKey.PubKey())
		accounts, err := ngstate.GetCurrentState().GetAccountsByPublicKey(key)
		if err != nil {
			return operations.NewGetAccountAllBadRequest().WithPayload(err.Error())
		}
		result := make([]interface{}, len(accounts))
		for i := range accounts {
			result[i] = accounts[i]
		}

		return operations.NewGetAccountAllOK().WithPayload(result)
	})

	api.GetBalanceMyHandler = operations.GetBalanceMyHandlerFunc(func(params operations.GetBalanceMyParams) middleware.Responder {
		return middleware.NotImplemented("operation operations.GetBalanceMy has not yet been implemented")
	})

	api.GetBlockHashOrHeightHandler = operations.GetBlockHashOrHeightHandlerFunc(func(params operations.GetBlockHashOrHeightParams) middleware.Responder {
		chain := storage.GetChain()

		height, err := strconv.ParseUint(params.HashOrHeight, 10, 64)
		if err != nil {
			// is hash
			hashHex := params.HashOrHeight

			hash, err := hex.DecodeString(hashHex)
			if err != nil {
				return operations.NewGetBlockHashOrHeightBadRequest().WithPayload(err.Error())
			}

			// is height
			block, err := chain.GetBlockByHash(hash)
			if err != nil {
				return operations.NewGetBlockHashOrHeightBadRequest().WithPayload(err.Error())
			}

			return operations.NewGetBlockHashOrHeightOK().WithPayload(block)
		}

		//
		block, err := chain.GetBlockByHeight(height)
		if err != nil {
			return operations.NewGetBlockHashOrHeightBadRequest().WithPayload(err.Error())
		}

		return operations.NewGetBlockHashOrHeightOK().WithPayload(block)

	})

	api.GetTxHashHandler = operations.GetTxHashHandlerFunc(func(params operations.GetTxHashParams) middleware.Responder {
		chain := storage.GetChain()
		hash, err := hex.DecodeString(params.Hash)
		if err != nil {
			return operations.NewGetTxHashBadRequest().WithPayload(err.Error())
		}

		tx, err := chain.GetTxByHash(hash)
		if err != nil {
			return operations.NewGetTxHashBadRequest().WithPayload(err.Error())
		}

		return operations.NewGetTxHashOK().WithPayload(tx)
	})

	api.GetTxpoolCheckHashHandler = operations.GetTxpoolCheckHashHandlerFunc(func(params operations.GetTxpoolCheckHashParams) middleware.Responder {
		return middleware.NotImplemented("operation operations.GetTxpoolCheckHash has not yet been implemented")
	})

	api.PostNodeAddrHandler = operations.PostNodeAddrHandlerFunc(func(params operations.PostNodeAddrParams) middleware.Responder {
		localNode := ngp2p.GetLocalNode()
		targetAddr, err := multiaddr.NewMultiaddr(params.Addr)
		if err != nil {
			return operations.NewPostNodeAddrBadRequest().WithPayload(err.Error())
		}

		targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
		if err != nil {
			return operations.NewPostNodeAddrBadRequest().WithPayload(err.Error())
		}
		err = localNode.Connect(context.Background(), *targetInfo)
		if err != nil {
			return operations.NewPostNodeAddrBadRequest().WithPayload(err.Error())
		}

		return operations.NewPostNodeAddrOK()
	})

	api.PostTxpoolSendHandler = operations.PostTxpoolSendHandlerFunc(func(params operations.PostTxpoolSendParams) middleware.Responder {
		err := txpool.GetTxPool().PutTxs(params.Tx.(*ngtypes.Tx))
		if err != nil {
			return operations.NewPostTxpoolSendBadRequest().WithPayload(err.Error())
		}
		return operations.NewPostTxpoolSendOK()
	})

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
