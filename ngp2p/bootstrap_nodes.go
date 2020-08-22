package ngp2p

import (
	"github.com/multiformats/go-multiaddr"
)

// BootstrapNodes is a list of all bootstrap nodes
var BootstrapNodes []multiaddr.Multiaddr

func init() {
	BootstrapNodes = make([]multiaddr.Multiaddr, 0) // dht.DefaultBootstrapPeers
	for _, s := range []string{
		"/dnsaddr/bootstrap.ngin.sh/p2p/16Uiu2HAmRZPWpQDGSqfWFKeH6cPRVKdhN3Q4Gyt1Xo86mr8rhwyu",
		"/dnsaddr/bootstrap.ngin.sh/p2p/16Uiu2HAmRCHKtLwjzRCju4fF1BDirzAKe6VAeZx8SsoZZKy49ox6",
	} {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			panic(err)
		}
		BootstrapNodes = append(BootstrapNodes, ma)
	}
}
