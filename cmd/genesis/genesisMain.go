package main

import (
	"flag"
	"log"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var regen = flag.Bool("regen", false, "regenerate the genesis.key")

func main() {
	flag.Parse()

	localKey := keytools.ReadLocalKey("genesis.key", "")
	if localKey == nil {
		if *regen {
			localKey = keytools.CreateLocalKey("genesis.key", "")
		} else {
			log.Panic("genesis.key is missing")
		}
	}

	raw := utils.ECDSAPublicKey2Bytes(localKey.PublicKey)
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

	// FIXME: before init network, manually init the R & S
	R, S, err := header.Signature(localKey)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Hex Generation R&S: %x %x", R.Bytes(), S.Bytes())
}
