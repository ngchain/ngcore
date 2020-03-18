# NgCore

New Golang implement of Ngin Network Node Daemon

## NGIN

NGIN is a totally new chain which is not a fork of ethereum or other chain. It is radically updating.

## requirements

go version >= 1.11

## Usage

```
./ngcore
./ngwallet newtx -to 1567464132546, 7563212343 -value 1NG, 0.1NG  
``` 

if you wanna start mining(PoW), try `--mining` flag

```
./ngcore --mining
```

## Build

### Go

```
go build ./cmd/ngcore
```

### Bazel

Bazel works better in linux than windows (personal experience)

```
bazel build //cmd/ngcore
```
