package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
	"github.com/urfave/cli/v2"
)

// contract-run executes a deterministic contract scenario against a local
// in-memory genesis fork and checks its expectations, exiting non-zero on
// any mismatch. It is the binary interface external contract projects
// (ngwasm, ngswap) use to test their wasm against the real ngcore VM —
// no Go test harness required on their side.
//
// Scenario JSON:
//
//	{
//	  "contracts": { "tokA": "path/to/ngtoken.wasm", "pair": "pair.wasm" },
//	  "steps": [ { "to": "pair", "by": "lp", "call": "setup",
//	               "args": ["@tokA", "@tokB"] }, ... ],
//	  "expect": [ { "in": "pair", "key": ["str:l", "@lp"], "u64": 2000000 } ]
//	}
//
// Byte-part DSL (args and expect.key): "@name" resolves to a 32-byte
// address (a contract's deploy address or a signer's key address),
// "u64:N" is an 8-byte little-endian integer, "str:X" is raw ASCII,
// "hex:XX" is raw hex.
type scenario struct {
	Contracts map[string]string `json:"contracts"`
	Steps     []struct {
		To   string   `json:"to"`
		By   string   `json:"by"`
		Call string   `json:"call"`
		Args []string `json:"args"`
		At   uint64   `json:"at"` // block timestamp for this step (default 1)
	} `json:"steps"`
	Expect []struct {
		In   string   `json:"in"`
		Key  []string `json:"key"`
		U64  uint64   `json:"u64"`
		U256 string   `json:"u256"` // decimal; compared as a 256-bit LE value
	} `json:"expect"`
}

func getContractTestCommand() *cli.Command {
	return &cli.Command{
		Name:      "contract-run",
		Usage:     "run a wasm contract test scenario against a local fork",
		ArgsUsage: "<scenario.json>",
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("usage: ngcore contract-run <scenario.json>")
			}
			return runScenario(c.Args().First())
		},
	}
}

func runScenario(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var sc scenario
	if err := json.Unmarshal(raw, &sc); err != nil {
		return fmt.Errorf("parse scenario: %w", err)
	}
	base := filepath.Dir(path)

	// resolve every named address: contracts by keccak("contract:"+name),
	// signers by their deterministic secp key's address
	addrs := map[string]ngtypes.Address{}
	keys := map[string]*ngtypes.PrivateKey{}
	for name := range sc.Contracts {
		addrs[name] = namedAddr("contract:" + name)
	}
	signer := func(name string) (*ngtypes.PrivateKey, ngtypes.Address, error) {
		if k, ok := keys[name]; ok {
			return k, addrs[name], nil
		}
		seed := utils.KeccakSum256([]byte("signer:" + name))
		k, err := ngtypes.NewKeyFromSeed(ngtypes.SchemeSecp256k1, seed)
		if err != nil {
			return nil, ngtypes.Address{}, err
		}
		a := ngtypes.NewAddress(k)
		keys[name], addrs[name] = k, a
		return k, a, nil
	}
	// resolve every referenced name up front: a "by" signer, or any
	// "@name" appearing in args/keys, becomes a deterministic signer
	// address unless it is already a deployed contract
	resolveName := func(name string) error {
		if _, ok := addrs[name]; ok {
			return nil
		}
		_, _, err := signer(name)
		return err
	}
	for _, st := range sc.Steps {
		if st.By != "" {
			if err := resolveName(st.By); err != nil {
				return err
			}
		}
		for _, p := range st.Args {
			if strings.HasPrefix(p, "@") {
				if err := resolveName(p[1:]); err != nil {
					return err
				}
			}
		}
	}
	for _, ex := range sc.Expect {
		for _, p := range ex.Key {
			if strings.HasPrefix(p, "@") {
				if err := resolveName(p[1:]); err != nil {
					return err
				}
			}
		}
	}

	// build the genesis fork with every contract deployed and active
	db, err := bbolt.Open(filepath.Join(os.TempDir(), fmt.Sprintf("ngcr-%d.db", os.Getpid())), 0o600, nil)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close(); _ = os.Remove(db.Path()) }()
	storage.InitDB(db)

	sheet := &ngtypes.Sheet{Network: ngtypes.ZERONET}
	for name, wasmPath := range sc.Contracts {
		if !filepath.IsAbs(wasmPath) {
			wasmPath = filepath.Join(base, wasmPath)
		}
		code, err := os.ReadFile(wasmPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", wasmPath, err)
		}
		acc := ngtypes.NewContract(addrs[name], code, nil)
		acc.SetActive(true)
		sheet.Contracts = append(sheet.Contracts, acc)
	}
	state := ngstate.InitStateFromSheet(db, ngtypes.ZERONET, sheet)

	// run the steps
	for i, st := range sc.Steps {
		target, ok := addrs[st.To]
		if !ok {
			return fmt.Errorf("step %d: unknown contract %q", i, st.To)
		}
		args, err := packParts(st.Args, addrs)
		if err != nil {
			return fmt.Errorf("step %d: %w", i, err)
		}
		var key *ngtypes.PrivateKey
		if st.By != "" {
			key, _, _ = signer(st.By)
		}

		acc, err := state.GetContract(target)
		if err != nil {
			return fmt.Errorf("step %d: no contract at %s: %w", i, st.To, err)
		}
		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1, target, nil, nil,
			ngtypes.EncodeCallData(st.Call, args), nil)
		if key != nil {
			if err := tx.Signature(key); err != nil {
				return err
			}
		}
		blockTime := st.At
		if blockTime == 0 {
			blockTime = 1
		}
		var gasUsed uint64
		if err := state.Update(func(txn *bbolt.Tx) error {
			vm, err := ngstate.NewVM(txn, acc, tx, blockTime)
			if err != nil {
				return err
			}
			if err := vm.Run(vm.EntryFor(ngstate.VMEntryOnTx)); err != nil {
				return err
			}
			gasUsed = vm.GasUsed()
			return nil
		}); err != nil {
			return fmt.Errorf("step %d (%s.%s): %w", i, st.To, st.Call, err)
		}
		fmt.Fprintf(os.Stderr, "step %d ok: %s.%s  (toll %d)\n", i, st.To, st.Call, gasUsed)
	}

	// check expectations
	fails := 0
	for _, ex := range sc.Expect {
		target, ok := addrs[ex.In]
		if !ok {
			return fmt.Errorf("expect: unknown contract %q", ex.In)
		}
		keyBytes, err := packParts(ex.Key, addrs)
		if err != nil {
			return err
		}
		acc, err := state.GetContract(target)
		if err != nil {
			return err
		}
		stored := acc.Context.Get(string(keyBytes))
		if ex.U256 != "" {
			want, ok := new(big.Int).SetString(ex.U256, 10)
			if !ok {
				return fmt.Errorf("expect: bad u256 %q", ex.U256)
			}
			got := new(big.Int).SetBytes(leToBE(stored))
			if got.Cmp(want) != 0 {
				fails++
				fmt.Fprintf(os.Stderr, "FAIL %s[%x] = %s, want %s\n", ex.In, keyBytes, got, want)
			}
			continue
		}
		got := leU64(stored)
		if got != ex.U64 {
			fails++
			fmt.Fprintf(os.Stderr, "FAIL %s[%x] = %d, want %d\n", ex.In, keyBytes, got, ex.U64)
		}
	}
	if fails > 0 {
		return fmt.Errorf("%d expectation(s) failed", fails)
	}
	fmt.Printf("ok  %s  (%d steps, %d checks)\n", filepath.Base(path), len(sc.Steps), len(sc.Expect))
	return nil
}

func namedAddr(s string) ngtypes.Address {
	var a ngtypes.Address
	copy(a[:], utils.KeccakSum256([]byte(s)))
	return a
}

func leU64(b []byte) uint64 {
	if len(b) < 8 {
		var buf [8]byte
		copy(buf[:], b)
		return binary.LittleEndian.Uint64(buf[:])
	}
	return binary.LittleEndian.Uint64(b)
}

// beToLE32 turns a big-endian big.Int byte slice into a 32-byte
// little-endian buffer (the U256 wire/storage encoding)
func beToLE32(be []byte) []byte {
	le := make([]byte, 32)
	for i, b := range be {
		le[len(be)-1-i] = b
	}
	return le
}

// leToBE reverses a little-endian value into big-endian for big.Int
func leToBE(le []byte) []byte {
	be := make([]byte, len(le))
	for i, b := range le {
		be[len(le)-1-i] = b
	}
	return be
}

// packParts turns the byte-part DSL into raw bytes
func packParts(parts []string, addrs map[string]ngtypes.Address) ([]byte, error) {
	var out []byte
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "@"):
			a, ok := addrs[p[1:]]
			if !ok {
				return nil, fmt.Errorf("unknown name %q", p)
			}
			out = append(out, a[:]...)
		case strings.HasPrefix(p, "u64:"):
			v, err := strconv.ParseUint(p[4:], 10, 64)
			if err != nil {
				return nil, err
			}
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], v)
			out = append(out, b[:]...)
		case strings.HasPrefix(p, "u256:"):
			v, ok := new(big.Int).SetString(p[5:], 10)
			if !ok || v.Sign() < 0 {
				return nil, fmt.Errorf("bad u256 %q", p)
			}
			out = append(out, beToLE32(v.Bytes())...)
		case strings.HasPrefix(p, "str:"):
			out = append(out, []byte(p[4:])...)
		case strings.HasPrefix(p, "hex:"):
			b, err := hex.DecodeString(p[4:])
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		default:
			return nil, fmt.Errorf("bad part %q", p)
		}
	}
	return out, nil
}
