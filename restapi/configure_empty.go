// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/restapi/operations"
	"github.com/ngchain/ngcore/utils"
)

//go:generate swagger generate server --target ../../ngcore --name  --spec ../swagger.json

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
	log := logging.Logger("rest")
	api.Logger = log.Debugf

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()
	api.GetHandler = operations.GetHandlerFunc(func(params operations.GetParams) middleware.Responder {
		return operations.NewGetOK().WithPayload("hello world")
	})
	api.GetAccountAllHandler = operations.GetAccountAllHandlerFunc(
		func(params operations.GetAccountAllParams) middleware.Responder {
			key := utils.PublicKey2Bytes(*consensus.GetConsensus().PrivateKey.PubKey())
			accounts, err := ngsheet.GetSheetManager().GetAccountsByPublicKey(key)
			if err != nil {
				return operations.NewGetAccountAllBadRequest()
			}
			result := make([]uint64, len(accounts))
			for i := range accounts {
				result[i] = accounts[i].Num
			}

			return operations.NewGetAccountAllOK()
		},
	)
	if api.GetAccountAtNumHandler == nil {
		api.GetAccountAtNumHandler = operations.GetAccountAtNumHandlerFunc(func(params operations.GetAccountAtNumParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetAccountAtNum has not yet been implemented")
		})
	}
	if api.GetAccountAtNumBalanceHandler == nil {
		api.GetAccountAtNumBalanceHandler = operations.GetAccountAtNumBalanceHandlerFunc(func(params operations.GetAccountAtNumBalanceParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetAccountAtNumBalance has not yet been implemented")
		})
	}
	if api.GetAccountMyHandler == nil {
		api.GetAccountMyHandler = operations.GetAccountMyHandlerFunc(func(params operations.GetAccountMyParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetAccountMy has not yet been implemented")
		})
	}
	if api.GetBalanceMyHandler == nil {
		api.GetBalanceMyHandler = operations.GetBalanceMyHandlerFunc(func(params operations.GetBalanceMyParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetBalanceMy has not yet been implemented")
		})
	}
	if api.GetBlockAtHeightHandler == nil {
		api.GetBlockAtHeightHandler = operations.GetBlockAtHeightHandlerFunc(func(params operations.GetBlockAtHeightParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetBlockAtHeight has not yet been implemented")
		})
	}
	if api.GetBlockHashHandler == nil {
		api.GetBlockHashHandler = operations.GetBlockHashHandlerFunc(func(params operations.GetBlockHashParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetBlockHash has not yet been implemented")
		})
	}
	if api.GetTxHashHandler == nil {
		api.GetTxHashHandler = operations.GetTxHashHandlerFunc(func(params operations.GetTxHashParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetTxHash has not yet been implemented")
		})
	}
	if api.GetTxpoolCheckHashHandler == nil {
		api.GetTxpoolCheckHashHandler = operations.GetTxpoolCheckHashHandlerFunc(func(params operations.GetTxpoolCheckHashParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.GetTxpoolCheckHash has not yet been implemented")
		})
	}
	if api.PostNodeAddrHandler == nil {
		api.PostNodeAddrHandler = operations.PostNodeAddrHandlerFunc(func(params operations.PostNodeAddrParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.PostNodeAddr has not yet been implemented")
		})
	}
	if api.PostTxpoolSendRawTxHandler == nil {
		api.PostTxpoolSendRawTxHandler = operations.PostTxpoolSendRawTxHandlerFunc(func(params operations.PostTxpoolSendRawTxParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.PostTxpoolSendRawTx has not yet been implemented")
		})
	}

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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		handler.ServeHTTP(w, r)
	})
}
