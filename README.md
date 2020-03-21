# NgCore

New Golang implement of Ngin Network Node Daemon

## NGIN

NGIN is a totally new chain which is not a fork of ethereum or other chain. It is radically updating.

## Requirements

go version >= 1.11

bazel build tool if you wanna use

## Usage

```bash
./ngcore
./ngwallet newtx -to 1567464132546, 7563212343 -value 1NG, 0.1NG  
``` 

if you wanna start mining(PoW), try `--mining` flag

```bash
./ngcore --mining
```

## Build

### Go

```bash
# go will automatically sync the dependencies
go build ./cmd/ngcore
```

**NOT RECOMMEND**: if you are under windows and **without `gcc`**, run `set CGO_ENABLED=0` or `go env -w CGO_ENABLED=0`(requires go>=1.13) before go build and then the build command will work fine.

### Bazel

Bazel works better in linux than windows (personal experience)

```bash
// BUILD.bazel files are not always updated with codes, it would be better update them (with gazelle)
bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore

// update repos from go.mod
bazel run //:gazelle -- update-repos -from_file=go.mod

// build the ngcore
bazel build //cmd/ngcore
```
