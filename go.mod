module github.com/ngchain/ngcore

go 1.14

require (
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e
	github.com/bytecodealliance/wasmtime-go v0.18.0
	github.com/cbergoon/merkletree v0.2.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/ipfs/go-log/v2 v2.1.1
	github.com/json-iterator/go v1.1.10
	github.com/libp2p/go-libp2p v0.10.0
	github.com/libp2p/go-libp2p-core v0.6.0
	github.com/libp2p/go-libp2p-kad-dht v0.8.2
	github.com/libp2p/go-libp2p-mplex v0.2.3
	github.com/libp2p/go-libp2p-pubsub v0.3.2
	github.com/libp2p/go-libp2p-yamux v0.2.8
	github.com/libp2p/go-tcp-transport v0.2.0
	github.com/maoxs2/go-jsonrpc2 v0.0.0-20200326130745-a6a35812420f
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/ngchain/go-randomx v0.1.1
	github.com/ngchain/go-schnorr v0.0.0-20200409140344-fdecf3cd59bd
	github.com/ngchain/secp256k1 v0.0.0-20200408111354-30fe4481b484
	github.com/urfave/cli/v2 v2.2.0
	go.uber.org/atomic v1.6.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/tools v0.0.0-20200702044944-0cc1aa72b347 // indirect
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

replace github.com/ngchain/go-randomx => ../go-randomx
