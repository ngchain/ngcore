module github.com/ngchain/ngcore

go 1.14

require (
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/btcsuite/btcd v0.21.0-beta // indirect
	github.com/c0mm4nd/go-jsonrpc2 v0.0.0-20201108125605-132876e6541e
	github.com/c0mm4nd/wasman v0.0.0-20201023051902-3f585a486d39
	github.com/cbergoon/merkletree v0.2.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/dgraph-io/ristretto v0.0.3 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/uuid v1.1.2
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/ipfs/go-log/v2 v2.1.1
	github.com/json-iterator/go v1.1.10
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/libp2p/go-libp2p v0.13.0
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20201026210036-4f868c957324 // indirect
	github.com/libp2p/go-libp2p-core v0.8.0 // dont upgrade to v0.7.x: https://github.com/libp2p/go-libp2p/pull/1001
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-mplex v0.4.1
	github.com/libp2p/go-libp2p-noise v0.1.2 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.4.0
	github.com/libp2p/go-libp2p-yamux v0.5.1
	github.com/libp2p/go-msgio v0.0.6
	github.com/libp2p/go-netroute v0.1.4 // indirect
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/libp2p/go-tcp-transport v0.2.1
	github.com/miekg/dns v1.1.35 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/ngchain/go-randomx v0.1.9
	github.com/ngchain/go-schnorr v0.0.0-20200409140344-fdecf3cd59bd
	github.com/ngchain/secp256k1 v0.0.0-20200408111354-30fe4481b484
	github.com/nxadm/tail v1.4.5 // indirect
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.4 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/urfave/cli/v2 v2.3.0
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/atomic v1.7.0
	go.uber.org/goleak v1.1.10 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/mod v0.4.0 // indirect
	golang.org/x/net v0.0.0-20201216054612-986b41b23924 // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a // indirect
	golang.org/x/sys v0.0.0-20201223074533-0d417f636930 // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/tools v0.0.0-20201224003254-57089f8fd729 // indirect
	google.golang.org/grpc v1.34.0 // indirect
	google.golang.org/protobuf v1.25.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.1.0 // indirect
)

replace github.com/ipfs/go-log/v2 v2.1.1 => github.com/ngchain/go-log/v2 v2.1.2-0.20201105165714-a4fff85de0e0
