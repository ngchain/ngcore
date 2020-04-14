package ngp2p

import (
	"context"
	"io/ioutil"
	"os"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

var log = logging.Logger("p2p")

type mdnsNotifee struct {
	h          host.Host
	ctx        context.Context
	PeerInfoCh chan peer.AddrInfo
}

func (m *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	_ = m.h.Connect(m.ctx, pi)
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
	_ = keyFile.Close()

	priv, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		log.Panic(err)
	}

	return priv
}

func getP2PKey() crypto.PrivKey {
	// read from db / file
	if _, err := os.Stat("p2p.key"); os.IsNotExist(err) {
		priv, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
		if err != nil {
			log.Panic(err)
		}

		raw, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			log.Panic(err)
		}

		log.Info("creating bootstrap key")
		f, err := os.Create("p2p.key")
		if err != nil {
			log.Panic(err)
		}

		_, _ = f.Write(raw)
		_ = f.Close()
	}

	return readKeyFromFile("p2p.key")
}
