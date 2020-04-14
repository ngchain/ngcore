module github.com/ngchain/ngcore

go 1.13

require (
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e
	github.com/cbergoon/merkletree v0.2.0
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/dlclark/regexp2 v1.2.0 // indirect
	github.com/dop251/goja v0.0.0-20200409105836-0cd29d81c048
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/golang/protobuf v1.4.0
	github.com/google/uuid v1.1.1
	github.com/ipfs/go-log v1.0.3
	github.com/json-iterator/go v1.1.9
	github.com/libp2p/go-libp2p v0.7.4
	github.com/libp2p/go-libp2p-core v0.5.1
	github.com/libp2p/go-libp2p-kad-dht v0.7.3
	github.com/libp2p/go-libp2p-mplex v0.2.3
	github.com/libp2p/go-libp2p-pubsub v0.2.7
	github.com/libp2p/go-libp2p-yamux v0.2.7
	github.com/libp2p/go-tcp-transport v0.2.0
	github.com/maoxs2/go-jsonrpc2 v0.0.0-20200326130745-a6a35812420f
	github.com/mr-tron/base58 v1.1.3
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/ngchain/cryptonight-go v0.0.0-20200408114201-bdcadf0ac3e1
	github.com/ngchain/go-schnorr v0.0.0-20200409140344-fdecf3cd59bd
	github.com/ngchain/secp256k1 v0.0.0-20200408111354-30fe4481b484
	github.com/urfave/cli/v2 v2.2.0
	github.com/whyrusleeping/go-logging v0.0.1
	go.uber.org/atomic v1.6.0
	go.uber.org/zap v1.14.1 // indirect
	golang.org/x/crypto v0.0.0-20200406173513-056763e48d71
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a // indirect
	google.golang.org/protobuf v1.21.0
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

replace github.com/ipfs/go-log v1.0.3 => github.com/maoxs2/go-log v1.0.3
