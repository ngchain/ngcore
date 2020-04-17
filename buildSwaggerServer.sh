swagger generate server -f swagger-ui/swagger.json
cd ./cmd/ngcore
go get -u github.com/gobuffalo/packr/v2/packr2
packr2
cd -

