package main

import (
	"fmt"
	"net/http"

	"github.com/go-openapi/loads"
	"github.com/gobuffalo/packr/v2"

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
	box := packr.New("swagger-ui", "../../swagger-ui/")
	fs := http.FileServer(box)
	http.Handle("/", fs)
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil); err != nil {
		panic(err)
	}
}
