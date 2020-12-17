package ngp2p

import (
	"github.com/multiformats/go-multiaddr"
)

// BootstrapNodes is a list of all bootstrap nodes
var BootstrapNodes []multiaddr.Multiaddr

func init() {
	BootstrapNodes = make([]multiaddr.Multiaddr, 0) // dht.DefaultBootstrapPeers
	for _, s := range []string{
		"/dnsaddr/bootstrap.ngin.sh/p2p/16Uiu2HAm8ZMX2dNvvsRb4amNZYTwd9XaPbtawCfStWVNMzSUgZhN",
		"/dnsaddr/bootstrap.ngin.sh/p2p/16Uiu2HAmRCHKtLwjzRCju4fF1BDirzAKe6VAeZx8SsoZZKy49ox6",
		"/dnsaddr/bootstrap.ngin.sh/p2p/16Uiu2HAmRv3oGGYKEUJfJbd7tYGUAy4Kq9y6H9voHupR4wBsktxW",
	} {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			panic(err)
		}
		BootstrapNodes = append(BootstrapNodes, ma)
	}
}
