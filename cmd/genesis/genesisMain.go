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

	gg := ngtypes.GetGenesisGenerateTx()

	// FIXME: before init network, manually init the R & S
	err := gg.Signature(localKey)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Hex Genesis Generate Tx R&S: %x %x", gg.R, gg.S)
}
