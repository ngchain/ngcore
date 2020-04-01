package ngp2p

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"io/ioutil"
	"os"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/whyrusleeping/go-logging"
)

var log = logging.MustGetLogger("p2p")

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

	// new one
	log.Infof("loading new p2p key")
	priv, _, _ := crypto.GenerateKeyPair(crypto.ECDSA, 256)
	return priv
}
