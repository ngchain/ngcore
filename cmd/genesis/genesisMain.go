package main

import (
	"flag"
	"log"

	"github.com/mr-tron/base58"

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

	raw := base58.FastBase58Encoding(utils.PublicKey2Bytes(*localKey.PubKey()))
	log.Printf("BS58 Genesis PublicKey: %s", raw)

	gg := ngtypes.GetGenesisGenerateTx()

	// FIXME: before init network, manually init the R & S
	err := gg.Signature(localKey)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("BS58 Genesis Generate Tx Sign: %s", base58.FastBase58Encoding(gg.Sign))
}
