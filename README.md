# NgCore
<p align="center" style="text-align: center">
<img width="400" height="400" src="./resources/NG.svg"/>
<br/>
<b>NgCore:</b> Brand-new golang daemon implement of Ngin Network Node
</p>

## What is NGIN?

NGIN is a radically updating **brand-new blockchain network**, which is not a fork of ethereum or any other chain.

NGIN's **goal** is to build **a blockchain engine** which acts more **auditable, scalable, security-oriented** and 
supports more network tasks with vm.

NGIN uses modern models - Vault(Block), Account, Multi-type Tx, and the traditional Block model to build the blockchain 
ecosystem. And NGIN strictly follow the idea of blockchain, locking items with hash to keep engine work safely.
Not only blockchain, but Vault(Block) will also link up to be a chain for account security and powerful functions like 
fast ignition, free account state and so on. So It's safe and robust like bitcoin and ethereum but more scalable and
 powerful on the node's operational capacity and p2p network's speed and performance. 

## Status

[![Go Report Card](https://goreportcard.com/badge/github.com/ngchain/ngcore)](
https://goreportcard.com/report/github.com/ngchain/ngcore)
[![CircleCI](https://circleci.com/gh/ngchain/ngcore.svg?style=svg)](https://circleci.com/gh/ngchain/ngcore)
![GitHub](https://img.shields.io/github/license/ngchain/ngcore)
![GitHub last commit](https://img.shields.io/github/last-commit/ngchain/ngcore)

## Features

- **Fast ignition**
- Less, or **no storage cost**(mem only)
- With **humanizing** account model, users can send tx with **memorable short number**
- **High security** with Sheet and Vault(Block) model
- Powerful and scalable types of tx
- Support **Multi-Tx**, sending coins to different places in the same time
- Powerful **WASM** VM support based on account state(contract).
- **Libp2p(ipfs)** powered p2p networking 
- Available **anonymous** address for saving balance
- Using the **schnorr signature**, allowing Multi-Sig when sending and receiving
- ...

## Requirements

go version >= 1.14

Or using bazel build tool if you want

Linux or WSL on Windows

## Build

### Go

```bash
# go will automatically sync the dependencies
# GCC is required because of high performance db & vm
go build ./cmd/ngcore
```

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

## Usage

```bash
# dircetly run the binary
./ngcore

# ngwallet is a rpc client in dart for ngin's daemon
./ngwallet newtx -to 1567464132546, 7563212343 -value 1NG, 0.1NG  
```

If you wanna start mining(proof of work), try `--mining` flag

```bash
./ngcore --mining
```

You can view more flags and options with `--help` flag
```bash
./ngcore --help
```

Or you can choose to run in a docker

```bash
git clone https://github.com/ngchain/ngcore && cd ngcore
sudo docker build . -t ngcore:alpine

# Run as a bootstrap node
sudo docker run -p 52520:52520 -p 52521:52521 -v ~/.ngcore:/workdir ngcore:alpine --bootstrap true

# Run as a mining node, 0 means using all cpu cores, --in-mem will disable writing into disk and make the miner lighter
sudo docker run -p 52520:52520 -p 52521:52521 -v ~/.ngcore:/workdir ngcore:alpine --mining 0 --in-mem
```

## Run a NGIN Forknet

1. Modify the Version & NetworkID in `./ngtypes/defaults.go` and `./ngp2p/methods.go`

2. Generate a new genesis key, a sign for genesis generate tx, and genesis block nonce (with `ngcore gen` tool)

3. Run a bootstrap node with `--bootstrap` flag (without mining)

4. Write the bootstrap node to bootstrapNodes in `./ngp2p/bootstraps.go`

5. Run a mining node with `--mining 0` flag
