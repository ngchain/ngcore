# BUILDER
FROM golang:alpine as builder

ARG goproxy=https://goproxy.io
ARG in_china=0
ENV GOPROXY ${goproxy}
ENV CHINA ${in_china}

COPY . /ngchain
WORKDIR /ngchain

# RUN apk add build-base
RUN if [ $CHINA == 1 ]; \
    then sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories; \
    fi
RUN apk add --no-cache make gcc musl-dev linux-headers git
RUN GOPROXY=$GOPROXY go build ./cmd/ngcore

# MAIN
FROM alpine:latest

COPY --from=builder /ngchain/ngcore /usr/local/bin/

WORKDIR /workdir

EXPOSE 52520 52521
ENTRYPOINT ["ngcore"]
