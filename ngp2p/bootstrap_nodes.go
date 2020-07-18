package ngp2p

import (
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
)

// BootstrapNodes is a list of all bootstrap nodes
var BootstrapNodes []multiaddr.Multiaddr

func init() {
	BootstrapNodes = dht.DefaultBootstrapPeers
	for _, s := range []string{
		"/ip4/134.175.71.218/tcp/52520/p2p/16Uiu2HAmRZPWpQDGSqfWFKeH6cPRVKdhN3Q4Gyt1Xo86mr8rhwyu",
		"/ip4/103.210.22.20/tcp/52520/p2p/16Uiu2HAmRCHKtLwjzRCju4fF1BDirzAKe6VAeZx8SsoZZKy49ox6",
	} {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			panic(err)
		}
		BootstrapNodes = append(BootstrapNodes, ma)
	}
}
