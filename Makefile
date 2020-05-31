docker-build:
	docker build -t ngcore . 
docker-build-china:
	docker build  -t ngcore --build-arg in_china=1 .
docker-run: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --in-mem --log-level debug
docker-run-china: docker-build-china
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --in-mem --log-level debug
docker-run-mining: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --mining 0 --in-mem --log-level debug
docker-run-mining-china: docker-build-china
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --mining 0 --in-mem --log-level debug
docker-run-bootstrap: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --bootstrap --in-mem --log-level debug
docker-run-bootstrap-china: docker-build-china
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --bootstrap --in-mem --log-level debug
build:
	go build ./cmd/ngcore
mining: build
	./ngcore --mining 0 --in-mem
bootstrap: build
	./ngcore --bootstrap --in-mem	
gen:
	go run ./cmd/ngcore gen
clean:
	rm ~/.ngdb
gen-swagger-server:
	rm -r restapi/operations
	swagger generate server -f swagger-ui/swagger.json
gazelle:
	bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore
	bazel run //:gazelle -- update-repos -from_file=go.mod
