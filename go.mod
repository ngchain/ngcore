module github.com/ngchain/ngcore

go 1.13

require (
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e
	github.com/bytecodealliance/wasmtime-go v0.15.0
	github.com/cbergoon/merkletree v0.2.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/go-openapi/errors v0.19.4
	github.com/go-openapi/loads v0.19.5
	github.com/go-openapi/runtime v0.19.15
	github.com/go-openapi/spec v0.19.7
	github.com/go-openapi/strfmt v0.19.5
	github.com/go-openapi/swag v0.19.9
	github.com/go-openapi/validate v0.19.8
	github.com/golang/protobuf v1.4.0
	github.com/google/uuid v1.1.1
	github.com/ipfs/go-log/v2 v2.0.4
	github.com/jessevdk/go-flags v1.4.0
	github.com/json-iterator/go v1.1.9
	github.com/libp2p/go-libp2p v0.8.0
	github.com/libp2p/go-libp2p-core v0.5.1
	github.com/libp2p/go-libp2p-kad-dht v0.7.4
	github.com/libp2p/go-libp2p-mplex v0.2.3
	github.com/libp2p/go-libp2p-pubsub v0.2.7
	github.com/libp2p/go-libp2p-yamux v0.2.7
	github.com/libp2p/go-tcp-transport v0.2.0
	github.com/maoxs2/go-jsonrpc2 v0.0.0-20200326130745-a6a35812420f
	github.com/mitchellh/mapstructure v1.2.2 // indirect
	github.com/mr-tron/base58 v1.1.3
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multibase v0.0.2 // indirect
	github.com/ngchain/cryptonight-go v0.0.0-20200408114201-bdcadf0ac3e1
	github.com/ngchain/go-schnorr v0.0.0-20200409140344-fdecf3cd59bd
	github.com/ngchain/secp256k1 v0.0.0-20200408111354-30fe4481b484
	github.com/rakyll/statik v0.1.7
	github.com/urfave/cli/v2 v2.2.0
	go.mongodb.org/mongo-driver v1.3.2 // indirect
	go.uber.org/atomic v1.6.0
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a // indirect
	golang.org/x/sys v0.0.0-20200413165638-669c56c373c4 // indirect
	golang.org/x/tools v0.0.0-20200417140056-c07e33ef3290 // indirect
	google.golang.org/protobuf v1.21.0
)

replace github.com/ipfs/go-log/v2 v2.0.4 => github.com/maoxs2/go-log/v2 v2.0.5-0.20200415042640-243636cd7aed
