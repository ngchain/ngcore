module github.com/ngchain/ngcore

go 1.13

require (
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e
	github.com/c0mm4nd/go-jsonrpc2 v0.0.0-20210730135302-66cb45a7fd88
	github.com/c0mm4nd/rlp v0.0.0-20210628165635-6ae77e058956
	github.com/c0mm4nd/wasman v0.0.0-20201023051902-3f585a486d39
	github.com/cbergoon/merkletree v0.2.0
	github.com/dgraph-io/badger/v3 v3.2011.1
	github.com/google/uuid v1.3.0
	github.com/ipfs/go-log/v2 v2.1.3
	github.com/json-iterator/go v1.1.11
	github.com/libp2p/go-libp2p v0.14.0
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-kad-dht v0.12.0
	github.com/libp2p/go-libp2p-mplex v0.4.1
	github.com/libp2p/go-libp2p-pubsub v0.4.1
	github.com/libp2p/go-libp2p-yamux v0.5.3
	github.com/libp2p/go-msgio v0.0.6
	github.com/libp2p/go-tcp-transport v0.2.2
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/ngchain/go-randomx v0.1.9
	github.com/ngchain/go-schnorr v0.0.0-20200409140344-fdecf3cd59bd
	github.com/ngchain/secp256k1 v0.0.0-20200408111354-30fe4481b484
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli/v2 v2.3.0
	go.uber.org/atomic v1.9.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
)

replace github.com/ipfs/go-log/v2 v2.1.3 => github.com/ngchain/go-log/v2 v2.1.2-0.20210526064208-fc6c91979746
