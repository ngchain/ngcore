# BUILDER
FROM golang:latest as builder

ARG goproxy=https://goproxy.io
ENV GOPROXY ${goproxy}

COPY . /build
WORKDIR /build

RUN GOPROXY=$GOPROXY go build ./cmd/ngcore

# MAIN
# FROM alpine:latest
FROM ubuntu:latest

COPY --from=builder /build/ngcore /usr/local/bin/

WORKDIR /.ngdb

EXPOSE 52520 52521
ENTRYPOINT ["ngcore"]
