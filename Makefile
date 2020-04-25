docker-build:
	docker build . -t ngcore
docker-mining: docker-build
	docker run ngcore --mining 0 --in-mem
build:
	go build ./cmd/ngcore
gen:
	go run ./cmd/ngcore gen
clean:
	rm .ngdb
