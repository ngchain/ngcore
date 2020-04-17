package main

import (
	"fmt"
	"net/http"

	"github.com/go-openapi/loads"

	"github.com/rakyll/statik/fs"

	_ "github.com/ngchain/ngcore/statik"

	"github.com/ngchain/ngcore/restapi"
	"github.com/ngchain/ngcore/restapi/operations"
)

func runSwaggerServer(port int) {
	swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
	if err != nil {
		panic(err)
	}

	api := operations.NewEmptyAPI(swaggerSpec)
	server := restapi.NewServer(api)
	defer server.Shutdown()

	server.ConfigureFlags()
	server.ConfigureAPI()
	server.Port = port

	if err := server.Serve(); err != nil {
		panic(err)
	}

}

func runSwaggerUI(host string, port int) {
	swaggerFs, err := fs.New()
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(swaggerFs)
	http.Handle("/", fileServer)
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil); err != nil {
		panic(err)
	}
}
