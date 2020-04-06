# BUILDER
FROM golang:alpine as builder

ENV GOPROXY https://goproxy.io

COPY . /ngchain
WORKDIR /ngchain

# RUN apk add build-base
RUN GOPROXY=$GOPROXY CGO_ENABLED=0 go build ./cmd/ngcore

# MAIN
FROM alpine:latest

COPY --from=builder /ngchain/ngcore /usr/local/bin/

WORKDIR /workdir

EXPOSE 52520 52521
ENTRYPOINT ["ngcore"]
