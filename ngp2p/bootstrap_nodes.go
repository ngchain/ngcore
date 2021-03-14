package ngp2p

import (
	"github.com/multiformats/go-multiaddr"
)

// BootstrapNodes is a list of all bootstrap nodes
var BootstrapNodes []multiaddr.Multiaddr

func init() {
	BootstrapNodes = make([]multiaddr.Multiaddr, 0) // dht.DefaultBootstrapPeers
	for _, s := range []string{
		"/dnsaddr/bootstrap.ngin.cash/p2p/16Uiu2HAm8ZMX2dNvvsRb4amNZYTwd9XaPbtawCfStWVNMzSUgZhN",
		"/dnsaddr/bootstrap.ngin.cash/p2p/16Uiu2HAmRCHKtLwjzRCju4fF1BDirzAKe6VAeZx8SsoZZKy49ox6",
		"/dnsaddr/bootstrap.ngin.cash/p2p/16Uiu2HAmRv3oGGYKEUJfJbd7tYGUAy4Kq9y6H9voHupR4wBsktxW",
		"/ip4/127.0.0.1/tcp/52520/p2p/16Uiu2HAm1HFhVjeLTmmhvmtRL4SdzNUdyJUxkiT3ekx6HwaXT7uc",
	} {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			panic(err)
		}
		BootstrapNodes = append(BootstrapNodes, ma)
	}
}
