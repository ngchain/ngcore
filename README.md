# NgCore

New Golang implement of Ngin Network Node Daemon

## NGIN

NGIN is a totally new chain which is not a fork of ethereum or other chain. It is radically updating.

## Status

[![DepShield Badge](https://depshield.sonatype.org/badges/ngchain/ngcore/depshield.svg)](https://depshield.github.io)
[![Go Report Card](https://goreportcard.com/badge/github.com/ngchain/ngcore)](https://goreportcard.com/report/github.com/ngchain/ngcore)
[![CircleCI](https://circleci.com/gh/ngchain/ngcore.svg?style=svg)](https://circleci.com/gh/ngchain/ngcore)

## Features

- Fast boot
- Little storage cost, or no cost(mem only)
- Humanizing account model
- Absolutely safety ensure with vault model
- Powerful types of tx
- Multi-language support on contract vm(lua & javascript yet)
- Pure go, support almost all platforms
- Libp2p(ipfs) powered p2p networking
- Account ecosystem to keep coin value
- ...

## Requirements

go version >= 1.13

Or using bazel build tool if you want 

## Usage

```bash
./ngcore
./ngwallet newtx -to 1567464132546, 7563212343 -value 1NG, 0.1NG  
``` 

If you wanna start mining(proof of work), try `--mining` flag

```bash
./ngcore --mining
```

## Build

### Go

```bash
# go will automatically sync the dependencies
go build ./cmd/ngcore
```

### Docker

```bash
sudo docker build . -t ngcore:alpine

# Run as a bootstrap node
sudo docker run -p 52520:52520 -p 52521:52521 -v ~/.ngcore:/workdir ngcore:alpine --bootstrap true

# Run as a mining node, 0 means using all cpu cores
sudo docker run -p 52520:52520 -p 52521:52521 -v ~/.ngcore:/workdir ngcore:alpine --mining 0
```

### Tip

Run `set CGO_ENABLED=0` or `go env -w CGO_ENABLED=0`(requires go>=1.13) before go build and then the build command will work fine when your environment don't have gcc.

### Bazel

Bazel works better in linux than windows (personal experience)

```bash
# BUILD.bazel files are not always updated with codes, it would be better update them (with gazelle)
bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore

# update repos from go.mod
bazel run //:gazelle -- update-repos -from_file=go.mod

# build the ngcore
bazel build //cmd/ngcore
```
