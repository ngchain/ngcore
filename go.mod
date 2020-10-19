module github.com/ngchain/ngcore

go 1.14

require (
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e
	github.com/c0mm4nd/wasman v0.0.0-20201014131632-77fad4c28570
	github.com/cbergoon/merkletree v0.2.0
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.2
	github.com/ipfs/go-log/v2 v2.1.1
	github.com/json-iterator/go v1.1.10
	github.com/libp2p/go-libp2p v0.11.0
	github.com/libp2p/go-libp2p-core v0.6.1 // dont upgrade to v0.7.x: https://github.com/libp2p/go-libp2p/pull/1001
	github.com/libp2p/go-libp2p-kad-dht v0.9.0
	github.com/libp2p/go-libp2p-mplex v0.2.4
	github.com/libp2p/go-libp2p-pubsub v0.3.6
	github.com/libp2p/go-libp2p-yamux v0.3.0
	github.com/libp2p/go-msgio v0.0.6
	github.com/libp2p/go-tcp-transport v0.2.1
	github.com/maoxs2/go-jsonrpc2 v0.0.0-20200715024857-a413889804df
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/ngchain/go-randomx v0.1.7
	github.com/ngchain/go-schnorr v0.0.0-20200409140344-fdecf3cd59bd
	github.com/ngchain/secp256k1 v0.0.0-20200408111354-30fe4481b484
	github.com/urfave/cli/v2 v2.2.0
	go.uber.org/atomic v1.7.0
	golang.org/x/crypto v0.0.0-20201012173705-84dcc777aaee
	google.golang.org/protobuf v1.25.0
)
