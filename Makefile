docker-build:
	docker build . -t ngcore
docker-mining: docker-build
	docker run ngcore --mining 0 --in-mem
docker-bootstrap: docker-build
	docker run ngcore --bootstrap --in-mem
build:
	go build ./cmd/ngcore
gen:
	go run ./cmd/ngcore gen
clean:
	rm .ngdb
swagger-server:
	swagger generate server -f swagger-ui/swagger.json
gazelle:
	bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore
	bazel run //:gazelle -- update-repos -from_file=go.mod
