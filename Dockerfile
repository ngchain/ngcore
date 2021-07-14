# Currently using ubuntu for usability.
# 
# Prerequisites for using the alpine image:
#   - add blockchain/RandomX musl binary release
#   - add blockchain/go-randomx musl binary release
#   - (add blockchain/ngcore musl binary release)

# BUILDER
FROM golang:latest as builder

ARG goproxy=https://goproxy.io
ENV GOPROXY ${goproxy}

COPY . /build
WORKDIR /build

RUN apt install gcc -y
RUN GOPROXY=$GOPROXY make build

# MAIN
FROM ubuntu:latest

COPY --from=builder /build/ngcore /usr/local/bin/

WORKDIR /workspace

EXPOSE 52520 52521
ENTRYPOINT ["ngcore"]
