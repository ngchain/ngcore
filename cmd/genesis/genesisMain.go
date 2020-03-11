package main

import (
	"crypto/elliptic"
	"flag"
	"github.com/ngin-network/ngcore/keyManager"
	"github.com/ngin-network/ngcore/ngtypes"
	"log"
)

var regen = flag.Bool("regen", false, "regenerate the genesis.key")

func main() {
	flag.Parse()

	keyMgr := keyManager.NewKeyManager("genesis.key", "")

	localKey := keyMgr.ReadLocalKey()
	if localKey == nil {
		if *regen {
			localKey = keyMgr.CreateLocalKey()
		} else {
			log.Panic("genesis.key is missing")
		}
	}
	raw := elliptic.Marshal(elliptic.P256(), localKey.X, localKey.Y)
	log.Printf("Hex Genesis PublicKey: %x", raw)
	header := &ngtypes.TxHeader{
		Version:      ngtypes.Version,
		Type:         0,
		Convener:     0,
		Participants: [][]byte{raw},
		Fee:          ngtypes.Big0Bytes,
		Values: [][]byte{
			ngtypes.OneBlockReward.Bytes(),
		},
		Nonce: 0, // block height 0
		Extra: nil,
	}

	// TODO: before init network should manually init the R & S
	R, S, err := header.Signature(localKey)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Hex Generation R&S: %x %x", R.Bytes(), S.Bytes())

}
