# BUILDER
FROM golang:alpine as builder

ARG goproxy=https://goproxy.io
ENV GOPROXY ${goproxy}

ARG in_china=0
ENV CHINA ${in_china}

COPY . /build
WORKDIR /build

RUN if [ $CHINA == 1 ]; \
    then sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories; \
    fi
RUN apk add --no-cache make gcc musl-dev linux-headers git
RUN GOPROXY=$GOPROXY go build ./cmd/ngcore

# MAIN
# FROM alpine:latest
FROM ubuntu:latest

COPY --from=builder /build/ngcore /usr/local/bin/

WORKDIR /workspace

EXPOSE 52520 52521
ENTRYPOINT ["ngcore"]
