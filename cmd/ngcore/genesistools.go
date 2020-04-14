package main

import (
	"math/big"
	"runtime"

	"github.com/NebulousLabs/fastrand"
	"github.com/mr-tron/base58"
	"github.com/ngchain/cryptonight-go"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var genesistoolsCommand = &cli.Command{
	Name:        "gen",
	Description: "built-in helper func for generate initial variables for genesis items",
	Action: func(context *cli.Context) error {
		localKey := keytools.ReadLocalKey("genesis.key", "")
		if localKey == nil {
			log.Panic("genesis.key is missing, using keytools to create one first")
		}

		raw := base58.FastBase58Encoding(utils.PublicKey2Bytes(*localKey.PubKey()))
		log.Warnf("BS58 Genesis PublicKey: %s", raw)

		gtx := ngtypes.GetGenesisGenerateTx()
		err := gtx.Signature(localKey)
		if err != nil {
			log.Panic(err)
		}

		log.Warnf("BS58 Genesis Generate Tx Sign: %s", base58.FastBase58Encoding(gtx.Sign))

		b := ngtypes.GetGenesisBlock()
		b, err = b.ToUnsealing([]*ngtypes.Tx{gtx})
		if err != nil {
			log.Error(err)
		}
		genBlockNonce(b)

		return nil
	},
}

func genBlockNonce(b *ngtypes.Block) {
	genesisTarget := new(big.Int).SetBytes(b.Header.Target)

	nCh := make(chan []byte, 1)
	stopCh := make(chan struct{}, 1)
	thread := runtime.NumCPU()

	for i := 0; i < thread; i++ {
		go calcHash(b, genesisTarget, nCh, stopCh)
	}

	answer := <-nCh
	stopCh <- struct{}{}
	blob := b.GetPoWBlob(answer)
	hash := cryptonight.Sum(blob, 0)
	log.Warnf("Nonce is %x, Hash is %x", answer, hash)
}

// calcHash get the hash of block
func calcHash(b *ngtypes.Block, target *big.Int, answerCh chan []byte, stopCh chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
			random := fastrand.Bytes(8)
			blob := b.GetPoWBlob(random)

			hash := cryptonight.Sum(blob, 0)
			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				answerCh <- random
				return
			}
		}
	}
}
