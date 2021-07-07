docker-build:
	docker build -t ngcore .
docker-run: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --in-mem --log-level debug
docker-run-bootstrap: docker-build
	docker run -p 52520:52520 -p 52521:52521 -v ~/.ngdb:/.ngdb ngcore --bootstrap --in-mem --log-level debug
build:
	go build -ldflags "-X main.Commit=`git rev-parse HEAD` -X main.Tag=`git describe --tags --abbrev=0`" ./cmd/ngcore
mining: build
	./ngcore --mining 0 --in-mem
bootstrap: build
	./ngcore --bootstrap --in-mem
clean:
	rm ~/ngdb
gazelle:
	bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore
	bazel run //:gazelle -- update-repos -from_file=go.mod
