docker-build:
	docker build -t ngcore . 
docker-build-china:
	docker build  -t ngcore --build-arg in_china=1 .  
docker-mining: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --mining 0 --in-mem --log-level debug
docker-mining-china: docker-build-china
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --mining 0 --in-mem --log-level debug
docker-bootstrap: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --bootstrap --in-mem --log-level debug
docker-bootstrap-china: docker-build-china
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
	rm ~/,.ngcore
gen-swagger-server:
	swagger generate server -f swagger-ui/swagger.json
gazelle:
	bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore
	bazel run //:gazelle -- update-repos -from_file=go.mod
