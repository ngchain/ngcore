package ngp2p

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-mplex"
	"github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/txpool"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type P2PServer struct {
}

func NewP2PServer() *P2PServer {
	return &P2PServer{}
}

type mdnsNotifee struct {
	h   host.Host
	ctx context.Context
}

func (m *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	m.h.Connect(m.ctx, pi)
}

func readKeyFromFile(filename string) crypto.PrivKey {
	keyFile, err := os.Open(filename)
	if err != nil {
		log.Panic(err)
	}

	raw, err := ioutil.ReadAll(keyFile)
	if err != nil {
		log.Panic(err)
	}
	keyFile.Close()

	ecdsaPriv, err := x509.ParseECPrivateKey(raw)
	if err != nil {
		log.Panic(err)
	}

	priv, _, _ := crypto.ECDSAKeyPairFromKey(ecdsaPriv)

	return priv
}

func getP2PKey(isBootstrap bool) crypto.PrivKey {
	if isBootstrap {
		// read from db / file
		if _, err := os.Stat("p2p.key"); os.IsNotExist(err) {
			priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				log.Panic(err)
			}

			raw, err := x509.MarshalECPrivateKey(priv)
			if err != nil {
				log.Panic(err)
			}

			log.Println("creating bootstrap key")
			f, err := os.Create("p2p.key")
			if err != nil {
				log.Panic(err)
			}

			f.Write(raw)
			f.Close()
		}

		return readKeyFromFile("p2p.key")
	}

	// new one
	log.Println("loading new p2p key")
	priv, _, _ := crypto.GenerateKeyPair(crypto.ECDSA, 256)
	return priv
}

func (s *P2PServer) Serve(port int, isBootstrap bool, sheetManager *sheetManager.SheetManager, blockChain *chain.BlockChain, vaultChain *chain.VaultChain, txPool *txpool.TxPool) {
	doneCh := make(chan bool, 1)
	priv := getP2PKey(isBootstrap)

	transports := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		//libp2p.Transport(ws.New),
	)

	listenAddrs := libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip6/::/tcp/%d", port),
	)
	ctx := context.Background()

	muxers := libp2p.ChainOptions(
		libp2p.Muxer("/yamux/1.0.0", sm_yamux.DefaultTransport),
		libp2p.Muxer("/mplex/6.7.0", peerstream_multiplex.DefaultTransport),
	)

	var ipfsDHT *dht.IpfsDHT
	newDHT := func(h host.Host) (routing.PeerRouting, error) {
		var err error
		ipfsDHT, err = dht.New(ctx, h)
		return ipfsDHT, err
	}

	localHost, err := libp2p.New(
		ctx,
		transports,
		listenAddrs,
		muxers,
		libp2p.Identity(priv),
		libp2p.Routing(newDHT),
	)
	if err != nil {
		panic(err)
	}

	// init
	for _, addr := range localHost.Addrs() {
		log.Println("Listening P2P on", addr.String()+"/p2p/"+localHost.ID().String())
	}

	localNode := NewNode(localHost, doneCh, sheetManager, blockChain, vaultChain, txPool)

	mdns, err := discovery.NewMdnsService(ctx, localHost, time.Second*10, "")
	if err != nil {
		panic(err)
	}
	mdns.RegisterNotifee(
		&mdnsNotifee{
			h:   localHost,
			ctx: ctx,
		},
	)

	err = ipfsDHT.Bootstrap(ctx)
	if err != nil {
		panic(err)
	}

	if !isBootstrap {
		for i := range BootstrapNodes {
			targetAddr, err := multiaddr.NewMultiaddr(BootstrapNodes[i])
			if err != nil {
				panic(err)
			}

			targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
			if err != nil {
				panic(err)
			}

			err = localHost.Connect(ctx, *targetInfo)
			if err != nil {
				panic(err)
			}

			localNode.Ping(targetInfo.ID)
		}
	}
}
