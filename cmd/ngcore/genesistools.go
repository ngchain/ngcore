package main

import (
	"fmt"
	"math/big"
	"runtime"
	"sync"

	"github.com/NebulousLabs/fastrand"
	"github.com/mr-tron/base58"
	"github.com/ngchain/go-randomx"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func getGenesisToolsCommand() *cli.Command {

	var filenameFlag = &cli.StringFlag{
		Name:    "filename",
		Aliases: []string{"f"},
		Value:   "genesis.key",
		Usage:   "the genesis.key file",
	}

	var passwordFlag = &cli.StringFlag{
		Name:    "password",
		Aliases: []string{"p"},
		Usage:   "the password to genesis.key file",
	}

	var checkCommand = &cli.Command{
		Name:        "check",
		Flags:       []cli.Flag{filenameFlag, passwordFlag},
		Description: "check genesis blocks and generateTx and re-generate them if error occurs",
		Action: func(context *cli.Context) error {
			filename := context.String("filename")
			password := context.String("password")

			localKey := keytools.ReadLocalKey(filename, password)
			if localKey == nil {
				err := fmt.Errorf("genesis.key is missing, using keytools to create one first")
				panic(err)
			}

			raw := base58.FastBase58Encoding(utils.PublicKey2Bytes(*localKey.PubKey()))
			fmt.Printf("Genesis PublicKey: %s \n", raw)

			fmt.Printf("Genesis Address: %s \n", ngtypes.NewAddress(localKey).String())

			for _, network := range ngtypes.AvailableNetworks {
				fmt.Printf("checking %s\n", network)

				gtx := ngtypes.GetGenesisGenerateTx(network)
				if err := gtx.CheckGenerate(); err != nil {
					fmt.Printf("current genesis generate tx sign %x is invalid, err: %s, resignaturing... \n", gtx.Sign, err)
					err = gtx.Signature(localKey)
					if err != nil {
						panic(err)
					}

					fmt.Printf("Genesis Generate Tx Sign: %x \n", gtx.Sign)
				} else {
					fmt.Printf("Genesis block's generate tx is healthy \n")
				}

				b := ngtypes.GetGenesisBlock(network)
				if err := b.CheckError(); err != nil {
					fmt.Printf("Current genesis block is invalid, err: %s, use the generate tx above to re-calc nonce...  \n", err)
					unsealing, err := b.ToUnsealing([]*ngtypes.Tx{gtx})
					if err != nil {
						fmt.Print(err)
					}

					genBlockNonce(unsealing)
				} else {
					fmt.Printf("Genesis block is healthy \n")
				}
			}

			return nil
		},
	}

	var displayCommand = &cli.Command{
		Name:        "display",
		Flags:       nil,
		Description: "check genesis blocks and generateTx and re-generate them if error occurs",
		Action: func(context *cli.Context) error {
			for _, network := range ngtypes.AvailableNetworks {
				b := ngtypes.GetGenesisBlock(network)
				jsonBlock, _ := utils.JSON.MarshalToString(b)
				fmt.Println(jsonBlock)
			}

			return nil
		},
	}

	return &cli.Command{
		Name:        "gentools",
		Flags:       nil,
		Description: "built-in helper func for generate initial variables for genesis items",
		Subcommands: []*cli.Command{checkCommand, displayCommand},
	}
}

func genBlockNonce(b *ngtypes.Block) {
	diff := new(big.Int).SetBytes(b.GetDifficulty())
	genesisTarget := new(big.Int).Div(ngtypes.MaxTarget, diff)
	fmt.Printf("Genesis block's diff %d, target %x \n", diff, genesisTarget.Bytes())

	nCh := make(chan []byte, 1)
	stopCh := make(chan struct{}, 1)
	thread := runtime.NumCPU()

	for i := 0; i < thread; i++ {
		go calcHash(b, genesisTarget, nCh, stopCh)
	}

	answer := <-nCh
	stopCh <- struct{}{}

	fmt.Printf("Genesis Block Nonce Hex: %x \n", answer)
}

func calcHash(b *ngtypes.Block, target *big.Int, answerCh chan []byte, stopCh chan struct{}) {
	// calcHash get the hash of block
	cache, err := randomx.AllocCache(randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	randomx.InitCache(cache, b.PrevBlockHash)
	ds, err := randomx.AllocDataset(randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	count := randomx.DatasetItemCount()
	var wg sync.WaitGroup
	var workerNum = uint32(runtime.NumCPU())
	for i := uint32(0); i < workerNum; i++ {
		wg.Add(1)
		a := (count * i) / workerNum
		b := (count * (i + 1)) / workerNum
		go func() {
			defer wg.Done()
			randomx.InitDataset(ds, cache, a, b-a)
		}()
	}
	wg.Wait()

	vm, err := randomx.CreateVM(cache, ds, randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case <-stopCh:
			return
		default:
			random := fastrand.Bytes(ngtypes.NonceSize)
			blob := b.GetPoWRawHeader(random)

			hash := randomx.CalculateHash(vm, blob)
			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				answerCh <- random
				return
			}
		}
	}
}
