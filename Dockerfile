# Currently using ubuntu for usability.
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
