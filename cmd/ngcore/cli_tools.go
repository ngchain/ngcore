package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// rpcCall posts one jsonrpc request and unwraps the result
func rpcCall(addr, method string, params any) ([]byte, error) {
	rawParams, err := utils.JSON.Marshal(params)
	if err != nil {
		return nil, err
	}

	c := jsonrpc2http.NewClient()
	msg := jsonrpc2.NewJsonRpcRequest(1, method, rawParams)
	request, err := jsonrpc2http.NewClientRequest(addr, msg)
	if err != nil {
		return nil, err
	}

	response, err := c.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	reply, err := jsonrpc2.UnmarshalMessage(raw)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid rpc reply: %s", raw)
	}
	if reply.Error != nil {
		return nil, errors.Errorf("%s: %s", method, reply.Error.Message)
	}
	if reply.Result == nil {
		return nil, nil
	}

	return *reply.Result, nil
}

var schemeByName = map[string]ngtypes.SigScheme{
	"secp256k1": ngtypes.SchemeSecp256k1,
	"fndsa512":  ngtypes.SchemeFNDSA512,
	"mldsa44":   ngtypes.SchemeMLDSA44,
	"slhdsa128": ngtypes.SchemeSLHDSA128,
}

var nameByScheme = map[ngtypes.SigScheme]string{
	ngtypes.SchemeSecp256k1: "secp256k1",
	ngtypes.SchemeFNDSA512:  "fndsa512",
	ngtypes.SchemeMLDSA44:   "mldsa44",
	ngtypes.SchemeSLHDSA128: "slhdsa128",
}

// signAndSend takes an unsigned tx hex from a gen* method, signs it
// LOCALLY (the key never leaves this machine) and broadcasts it
func signAndSend(ctx *cli.Context, unsignedRaw []byte) error {
	var unsignedHex string
	if err := utils.JSON.Unmarshal(unsignedRaw, &unsignedHex); err != nil {
		return err
	}

	rawTx, err := hex.DecodeString(unsignedHex)
	if err != nil {
		return err
	}

	var tx ngtypes.FullTx
	if err := rlp.DecodeBytes(rawTx, &tx); err != nil {
		return err
	}

	key := keytools.ReadLocalKey(ctx.String("key"), ctx.String("password"))
	if err := tx.Signature(key); err != nil {
		return err
	}

	signedRaw, err := rlp.EncodeToBytes(&tx)
	if err != nil {
		return err
	}

	result, err := rpcCall(ctx.String("addr"), "ng_sendTx",
		map[string]any{"rawTx": hex.EncodeToString(signedRaw)})
	if err != nil {
		return err
	}

	fmt.Printf("tx sent: %s\n", result)
	return nil
}

func genThenSend(ctx *cli.Context, method string, params map[string]any) error {
	unsigned, err := rpcCall(ctx.String("addr"), method, params)
	if err != nil {
		return err
	}

	return signAndSend(ctx, unsigned)
}

func ownAddress(ctx *cli.Context) ngtypes.Address {
	key := keytools.ReadLocalKey(ctx.String("key"), ctx.String("password"))
	return ngtypes.NewAddress(key)
}

func printJSON(raw []byte) {
	fmt.Println(string(raw))
}

func getCliToolsCommand() *cli.Command {
	return &cli.Command{
		Name:        "cli",
		Description: "wallet and rpc client (keys stay local; only signed txs travel)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Aliases: []string{"a"},
				Usage:   "the daemon rpc server address",
				Value:   "http://localhost:52521",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "path to the key file (empty = the default one)",
			},
			&cli.StringFlag{
				Name:  "password",
				Usage: "password of the key file",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:        "key",
				Description: "show the local key: scheme and address; --new creates one",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "new", Usage: "create a fresh key file"},
					&cli.StringFlag{
						Name:  "scheme",
						Usage: "signature scheme for --new: secp256k1 | fndsa512 | mldsa44 | slhdsa128",
						Value: "secp256k1",
					},
				},
				Action: func(ctx *cli.Context) error {
					if ctx.Bool("new") {
						scheme, ok := schemeByName[ctx.String("scheme")]
						if !ok {
							return errors.Errorf("unknown scheme %q", ctx.String("scheme"))
						}
						key := keytools.CreateLocalKeyWithScheme(ctx.String("key"), ctx.String("password"), scheme)
						fmt.Printf("scheme:  %s\naddress: %s\n", nameByScheme[key.Scheme], ngtypes.NewAddress(key))
						return nil
					}

					key := keytools.ReadLocalKey(ctx.String("key"), ctx.String("password"))
					fmt.Printf("scheme:  %s\naddress: %s\n", nameByScheme[key.Scheme], ngtypes.NewAddress(key))
					return nil
				},
			},
			{
				Name:        "status",
				Description: "chain status: network and the latest block",
				Action: func(ctx *cli.Context) error {
					for _, method := range []string{"net_getNetwork", "ng_getLatestBlockHeight", "ng_getLatestBlockHash"} {
						raw, err := rpcCall(ctx.String("addr"), method, nil)
						if err != nil {
							return err
						}
						fmt.Printf("%s: %s\n", method, raw)
					}
					return nil
				},
			},
			{
				Name:        "balance",
				Description: "balance of an address (default: the local key's)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "of", Usage: "bs58 address (empty = own)"},
				},
				Action: func(ctx *cli.Context) error {
					address := ctx.String("of")
					if address == "" {
						address = ownAddress(ctx).BS58()
					}
					raw, err := rpcCall(ctx.String("addr"), "ng_getBalanceByAddress",
						map[string]any{"address": address})
					if err != nil {
						return err
					}
					fmt.Printf("address: %s\n", address)
					printJSON(raw)
					return nil
				},
			},
			{
				Name:        "send",
				Description: "pay an address (optionally calling a contract entry)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "to", Required: true, Usage: "recipient bs58 address"},
					&cli.StringFlag{Name: "value", Usage: "amount in NG, decimal string (exact)"},
					&cli.StringFlag{Name: "fee", Usage: "tx fee in NG, decimal string (exact)"},
					&cli.StringFlag{Name: "entry", Usage: "contract export to call (by name)"},
					&cli.StringFlag{Name: "args", Usage: "hex args for the entry"},
				},
				Action: func(ctx *cli.Context) error {
					return genThenSend(ctx, "ng_genTransaction", map[string]any{
						"to":    ctx.String("to"),
						"value": ctx.String("value"),
						"fee":   ctx.String("fee"),
						"entry": ctx.String("entry"),
						"extra": ctx.String("args"),
					})
				},
			},
			{
				Name:        "commit",
				Description: "commit a compiled wasm module onto the own contract slot: the node diffs it against the on-chain binary and carries the minimal patch (the first commit deploys)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Required: true, Usage: "path to the compiled contract module (.wasm)"},
					&cli.StringFlag{Name: "fee", Usage: "tx fee in NG, decimal string (exact)"},
				},
				Action: func(ctx *cli.Context) error {
					module, err := os.ReadFile(ctx.String("file"))
					if err != nil {
						return err
					}
					return genThenSend(ctx, "ng_genCommit", map[string]any{
						"fee":  ctx.String("fee"),
						"wasm": hex.EncodeToString(module),
					})
				},
			},
			{
				Name:        "activate",
				Description: "freeze the own contract source and turn its vm on (runs init once)",
				Flags:       []cli.Flag{&cli.StringFlag{Name: "fee", Usage: "tx fee in NG, decimal string (exact)"}},
				Action: func(ctx *cli.Context) error {
					return genThenSend(ctx, "ng_genActivate", map[string]any{"fee": ctx.String("fee")})
				},
			},
			{
				Name:        "deactivate",
				Description: "turn the own contract vm off and reopen the source for commits",
				Flags:       []cli.Flag{&cli.StringFlag{Name: "fee", Usage: "tx fee in NG, decimal string (exact)"}},
				Action: func(ctx *cli.Context) error {
					return genThenSend(ctx, "ng_genDeactivate", map[string]any{"fee": ctx.String("fee")})
				},
			},
			{
				Name:        "destroy",
				Description: "remove the own contract slot (source AND storage)",
				Flags:       []cli.Flag{&cli.StringFlag{Name: "fee", Usage: "tx fee in NG, decimal string (exact)"}},
				Action: func(ctx *cli.Context) error {
					return genThenSend(ctx, "ng_genDestroy", map[string]any{"fee": ctx.String("fee")})
				},
			},
			{
				Name:        "contract",
				Description: "inspect a contract slot (default: the own one)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "of", Usage: "deployer bs58 address (empty = own)"},
				},
				Action: func(ctx *cli.Context) error {
					address := ctx.String("of")
					if address == "" {
						address = ownAddress(ctx).BS58()
					}
					raw, err := rpcCall(ctx.String("addr"), "ng_getContractInfo",
						map[string]any{"address": address})
					if err != nil {
						return err
					}
					printJSON(raw)
					return nil
				},
			},
			{
				Name:        "call",
				Description: "dry-run a contract entry against the current state (free, nothing lands on chain)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "contract", Required: true, Usage: "deployer bs58 address"},
					&cli.StringFlag{Name: "entry", Usage: "entry export (empty = main)"},
					&cli.StringFlag{Name: "args", Usage: "raw args string"},
					&cli.StringFlag{Name: "value", Usage: "simulated payment in NG, decimal string"},
				},
				Action: func(ctx *cli.Context) error {
					raw, err := rpcCall(ctx.String("addr"), "ng_callContract", map[string]any{
						"contract": ctx.String("contract"),
						"entry":    ctx.String("entry"),
						"extra":    ctx.String("args"),
						"value":    ctx.String("value"),
					})
					if err != nil {
						return err
					}
					printJSON(raw)
					return nil
				},
			},
			{
				Name:        "tx",
				Description: "look a tx up by hash, with its receipt",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "hash", Required: true, Usage: "hex tx hash"},
				},
				Action: func(ctx *cli.Context) error {
					for _, method := range []string{"ng_getTxByHash", "ng_getReceipt"} {
						raw, err := rpcCall(ctx.String("addr"), method,
							map[string]any{"hash": ctx.String("hash")})
						if err != nil {
							return err
						}
						fmt.Printf("%s:\n", method)
						printJSON(raw)
					}
					return nil
				},
			},
		},
	}
}
