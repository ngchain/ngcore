package main

import (
	"fmt"
	"math/big"
	"runtime"

	"github.com/NebulousLabs/fastrand"
	logging "github.com/ipfs/go-log/v2"
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
		logging.SetAllLoggers(logging.LevelDebug)

		localKey := keytools.ReadLocalKey("genesis.key", "") // TODO: add password to options
		if localKey == nil {
			err := fmt.Errorf("genesis.key is missing, using keytools to create one first")
			log.Panic(err)
			return err
		}

		raw := base58.FastBase58Encoding(utils.PublicKey2Bytes(*localKey.PubKey()))
		log.Warnf("BS58 Genesis PublicKey: %s", raw)

		log.Warnf("BS58 Genesis Address: %s", ngtypes.NewAddress(localKey).String())

		gtx := ngtypes.GetGenesisGenerateTx()
		if err := gtx.CheckGenerate(); err != nil {
			log.Warnf("current genesis generate tx sign %x is invalid, err: %s, resignaturing...", gtx.Sign, err)
			err = gtx.Signature(localKey)
			if err != nil {
				log.Panic(err)
			}

			log.Warnf("BS58 Genesis Generate Tx Sign: %x", gtx.Sign)
		} else {
			log.Info("genesis block's generate tx is healthy")
		}

		b := ngtypes.GetGenesisBlock()
		if err := b.CheckError(); err != nil {
			log.Warnf("current genesis block is invalid, use the generate tx above to re-calc nonce...")
			b, err := b.ToUnsealing([]*ngtypes.Tx{gtx})
			if err != nil {
				log.Error(err)
			}

			genBlockNonce(b)
		} else {
			log.Info("genesis block is healthy")
		}

		return nil
	},
}

func genBlockNonce(b *ngtypes.Block) {
	diff := new(big.Int).SetBytes(b.GetDifficulty())
	genesisTarget := new(big.Int).Div(ngtypes.MaxTarget, diff)

	nCh := make(chan []byte, 1)
	stopCh := make(chan struct{}, 1)
	thread := runtime.NumCPU()

	for i := 0; i < thread; i++ {
		go calcHash(b, genesisTarget, nCh, stopCh)
	}

	answer := <-nCh
	stopCh <- struct{}{}

	log.Warnf("Genesis Block Nonce Hex: %x", answer)
}

func calcHash(b *ngtypes.Block, target *big.Int, answerCh chan []byte, stopCh chan struct{}) {
	// calcHash get the hash of block
	for {
		select {
		case <-stopCh:
			return
		default:
			random := fastrand.Bytes(ngtypes.NonceSize)
			blob := b.GetPoWBlob(random)

			hash := cryptonight.Sum(blob, 0)
			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				answerCh <- random
				return
			}
		}
	}
}
