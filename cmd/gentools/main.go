package main

import (
	"fmt"
	"os"

	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
	"github.com/urfave/cli/v2"
)

const (
	usage       = "Helper for generating initial variables for genesis items in ngchain"
	description = ""
	version     = ""
)

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
		fmt.Printf("genesis public key: %s \n", raw)

		fmt.Printf("genesis Address: %s \n", ngtypes.NewAddress(localKey).String())

		for _, network := range ngtypes.AvailableNetworks {
			fmt.Printf("checking %s\n", network)

			gtx := ngtypes.GetGenesisGenerateTx(network)
			if err := gtx.CheckGenerate(0); err != nil {
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
				err := b.ToUnsealing([]*ngtypes.Tx{gtx})
				if err != nil {
					fmt.Print(err)
				}

				genBlockNonce(b)
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

func main() {
	app := cli.NewApp()

	app.Name = "genesisutil"
	app.Usage = usage
	app.Description = description
	app.Version = version
	app.Action = nil
	app.Commands = []*cli.Command{checkCommand, displayCommand}

	app.Flags = []cli.Flag{filenameFlag, passwordFlag}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	os.Exit(0)
}
