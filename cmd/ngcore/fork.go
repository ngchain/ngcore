package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/c0mm4nd/rlp"
	logging "github.com/ngchain/zap-log"
	"github.com/urfave/cli/v2"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

var forkLog = logging.Logger("fork")

// `ngcore fork` is an anvil-style fork-chain tool for debugging
// contracts: it FORKS a chain's state and serves an instantly-sealing
// local copy over JSON-RPC. No PoW, no p2p — every submitted tx goes
// through the REAL state-transition path (HandleTxs: fees, balances,
// contract lifecycle, receipts, block gas), it just seals immediately.
//
// Fork sources:
//   - --rpc <url>: fork a RUNNING node over its JSON-RPC, LAZILY (anvil
//     style): boot reads only the head (getHead); every address —
//     balance, contract code + context — is fetched on FIRST TOUCH
//     (getAddressState) through ngstate's read-through fallback and
//     materialized locally, so forking scales with what the debug
//     session actually reads, not with chain size. --eager instead pulls
//     the whole state in one getSheet call (small chains, offline work).
//   - --db <path>: fork a local node's database (works on a copy)
//   - neither: a throwaway fresh genesis chain
//
// Named identities: every dev address (dev0..devN, or dev_newAddress) and
// every deployed contract registers under "@name"; RPC params accept
// "@name" or a bs58 address anywhere an address is expected. Keys derive
// deterministically from keccak("signer:"+name) — the SAME derivation
// contract-run uses, so scenarios and interactive debugging agree.
type forkChain struct {
	mu sync.Mutex

	network ngtypes.Network
	dbPath  string
	db      *bbolt.DB
	state   *ngstate.State

	height    uint64
	blockTime uint64

	names map[string]ngtypes.Address
	keys  map[string]*ngtypes.PrivateKey

	fund      *big.Int // raw units credited to every new dev address
	snapshots map[int]devSnapshot
	nextSnap  int
}

type devSnapshot struct {
	path      string
	height    uint64
	blockTime uint64
}

func getForkCommand() *cli.Command {
	return &cli.Command{
		Name:  "fork",
		Usage: "fork a chain (over RPC or from a db) into an instant-seal local copy for contract debugging",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "rpc", Usage: "fork a RUNNING node over JSON-RPC (lazy per-address fetch)"},
			&cli.BoolFlag{Name: "eager", Usage: "with --rpc: pull the whole state up front (one getSheet) instead of lazily"},
			&cli.StringFlag{Name: "db", Usage: "fork a local chain db (works on a copy)"},
			&cli.StringFlag{Name: "listen", Value: "127.0.0.1:52521", Usage: "local JSON-RPC listen address"},
			&cli.IntFlag{Name: "addresses", Value: 10, Usage: "number of prefunded dev addresses"},
			&cli.StringFlag{Name: "fund", Value: "1000000000000000000000000", Usage: "raw units per dev address (default 1M NG)"},
			&cli.Uint64Flag{Name: "time", Value: 1_000_000, Usage: "starting block timestamp (fresh/db modes)"},
		},
		Action: func(c *cli.Context) error {
			return runFork(c)
		},
	}
}

func runFork(c *cli.Context) error {
	fund, ok := new(big.Int).SetString(c.String("fund"), 10)
	if !ok {
		return fmt.Errorf("bad --fund %q", c.String("fund"))
	}

	d := &forkChain{
		network:   ngtypes.ZERONET,
		height:    1,
		blockTime: c.Uint64("time"),
		names:     map[string]ngtypes.Address{},
		keys:      map[string]*ngtypes.PrivateKey{},
		fund:      fund,
		snapshots: map[int]devSnapshot{},
		nextSnap:  1,
	}

	// the working db is always throwaway: the fork source is read once
	// and never touched again. Snapshot copies go with it on exit
	d.dbPath = filepath.Join(os.TempDir(), fmt.Sprintf("ngfork-%d.db", os.Getpid()))
	cleanup := func() {
		_ = os.Remove(d.dbPath)
		for _, snap := range d.snapshots {
			_ = os.Remove(snap.path)
		}
	}
	defer cleanup()
	// the server blocks until a signal kills the process, so the defer
	// alone would never run: catch ctrl-c/TERM and sweep the temp files
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		d.mu.Lock() // let an in-flight call finish
		cleanup()
		os.Exit(0)
	}()

	mode := "fresh genesis"
	switch {
	case c.String("rpc") != "" && c.String("db") != "":
		return fmt.Errorf("--rpc and --db are exclusive")

	case c.String("rpc") != "" && c.Bool("eager"):
		// eager rpc fork: ONE getSheet call captures the remote node's
		// full state; rebuild it locally at the same height/time
		sheet, timestamp, err := fetchRemoteSheet(c.String("rpc"))
		if err != nil {
			return fmt.Errorf("rpc fork %s: %w", c.String("rpc"), err)
		}
		if err := d.open(); err != nil {
			return err
		}
		d.network = sheet.Network
		d.state = ngstate.InitStateFromSheet(d.db, sheet.Network, sheet)
		d.height = sheet.Height + 1
		d.blockTime = timestamp
		mode = fmt.Sprintf("rpc fork (eager) of %s (height %d, %d contracts, %d balances)",
			c.String("rpc"), sheet.Height, len(sheet.Contracts), len(sheet.Balances))

	case c.String("rpc") != "":
		// LAZY rpc fork (the default): boot reads only the head; every
		// address materializes on first touch through ngstate's
		// read-through fallback, so forking cost tracks what the debug
		// session actually reads — chain size does not matter
		head, err := fetchRemoteHead(c.String("rpc"))
		if err != nil {
			return fmt.Errorf("rpc fork %s: %w", c.String("rpc"), err)
		}
		if err := d.open(); err != nil {
			return err
		}
		d.network = ngtypes.Network(head.Network)
		d.state = ngstate.InitStateFromSheet(d.db, d.network, &ngtypes.Sheet{Network: d.network})
		d.height = head.Height + 1
		d.blockTime = head.Timestamp
		d.installLazyFetcher(c.String("rpc"))
		mode = fmt.Sprintf("rpc fork (lazy) of %s at height %d — state fetches on first touch",
			c.String("rpc"), head.Height)

	case c.String("db") != "":
		if err := copyFile(c.String("db"), d.dbPath); err != nil {
			return fmt.Errorf("db fork copy: %w", err)
		}
		if err := d.open(); err != nil {
			return err
		}
		mode = fmt.Sprintf("db fork of %s (on a copy)", c.String("db"))

	default:
		if err := d.open(); err != nil {
			return err
		}
		// fresh genesis fork (same base contract-run uses)
		d.state = ngstate.InitStateFromSheet(d.db, d.network, &ngtypes.Sheet{Network: d.network})
	}
	defer func() { _ = d.db.Close() }()

	// prefund the deterministic dev addresses through the real
	// GenerateTx path (this also registers their pubkeys on chain)
	for i := 0; i < c.Int("addresses"); i++ {
		name := fmt.Sprintf("dev%d", i)
		if _, err := d.newAddress(name); err != nil {
			return err
		}
	}

	fmt.Printf("ngcore fork — instant-seal fork chain (no PoW)\n")
	fmt.Printf("  source: %s\n", mode)
	fmt.Printf("  rpc:    http://%s\n", c.String("listen"))
	fmt.Printf("  db:     %s\n", d.dbPath)
	fmt.Printf("  block:  height %d, time %d (+1s per sealed tx)\n\n", d.height, d.blockTime)
	fmt.Printf("addresses (keys derive from keccak(\"signer:\"+name), same as contract-run):\n")
	for i := 0; i < c.Int("addresses"); i++ {
		name := fmt.Sprintf("dev%d", i)
		fmt.Printf("  @%-6s %s  (%s raw)\n", name, d.names[name].String(), d.fund)
	}
	fmt.Printf("\nmethods: dev_addresses dev_newAddress dev_deploy dev_call dev_kv\n")
	fmt.Printf("         dev_balance dev_mine dev_setTime dev_snapshot dev_revert\n")
	fmt.Printf("example:\n")
	fmt.Printf("  curl -s http://%s -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"dev_deploy\",\"params\":{\"name\":\"token\",\"path\":\"ngtoken.wasm\"}}'\n", c.String("listen"))
	fmt.Printf("  curl -s http://%s -d '{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"dev_call\",\"params\":{\"to\":\"@token\",\"by\":\"@dev0\",\"method\":\"mint\",\"args\":[\"@dev0\",\"u256:1000000000000000000\"]}}'\n\n", c.String("listen"))

	server := jsonrpc2http.NewServer(jsonrpc2http.ServerConfig{
		Addr:   c.String("listen"),
		Logger: forkLog,
	})
	d.register(server)

	forkLog.Warnf("fork-chain JSON-RPC listening on %s", c.String("listen"))
	return server.ListenAndServe()
}

// rpcPost fires one JSON-RPC call at the remote node and returns the
// raw `result` bytes (an error when the node answered with one)
func rpcPost(url, method, params string) ([]byte, error) {
	if params == "" {
		params = "{}"
	}
	req := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(req)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var reply struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := utils.JSON.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("bad %s response: %w", method, err)
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("remote node: %s", reply.Error.Message)
	}
	if len(reply.Result) == 0 {
		return nil, fmt.Errorf("remote node returned no result (does it support %s?)", method)
	}
	return reply.Result, nil
}

// remoteHead is the light boot info of a lazy rpc fork
type remoteHead struct {
	Network   uint8  `json:"network"`
	Height    uint64 `json:"height"`
	Timestamp uint64 `json:"timestamp"`
}

func fetchRemoteHead(url string) (*remoteHead, error) {
	raw, err := rpcPost(url, "getHead", "")
	if err != nil {
		return nil, err
	}
	var head remoteHead
	if err := utils.JSON.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	return &head, nil
}

// fetchRemoteSheet forks a running node over JSON-RPC eagerly: one
// `getSheet` call returns the full state (hex RLP) plus the head
// height/timestamp
func fetchRemoteSheet(url string) (*ngtypes.Sheet, uint64, error) {
	raw, err := rpcPost(url, "getSheet", "")
	if err != nil {
		return nil, 0, err
	}

	var result struct {
		Height    uint64 `json:"height"`
		Timestamp uint64 `json:"timestamp"`
		Sheet     string `json:"sheet"`
	}
	if err := utils.JSON.Unmarshal(raw, &result); err != nil {
		return nil, 0, fmt.Errorf("bad getSheet result: %w", err)
	}

	rawSheet, err := hex.DecodeString(result.Sheet)
	if err != nil {
		return nil, 0, err
	}
	var sheet ngtypes.Sheet
	if err := rlp.DecodeBytes(rawSheet, &sheet); err != nil {
		return nil, 0, fmt.Errorf("decode sheet: %w", err)
	}

	return &sheet, result.Timestamp, nil
}

// installLazyFetcher wires ngstate's read-through fallback to the remote
// node: on the FIRST local miss of an address its state (balance +
// contract with code and storage) is pulled via getAddressState and
// materialized locally; every address is fetched at most once (negative
// results cache too), so the remote sees one small request per address
// the debug session actually touches
func (d *forkChain) installLazyFetcher(url string) {
	type cached struct {
		acc   *ngtypes.Contract
		bal   *big.Int
		found bool
	}
	cache := map[ngtypes.Address]cached{} // single-goroutine: d.mu serializes all rpc handlers

	ngstate.SetRemoteFallback(func(addr ngtypes.Address) (*ngtypes.Contract, *big.Int, bool) {
		if hit, ok := cache[addr]; ok {
			return hit.acc, hit.bal, hit.found
		}
		entry := cached{}
		cache[addr] = entry // negative until proven otherwise (no refetch storms)

		raw, err := rpcPost(url, "getAddressState", fmt.Sprintf(`{"address":%q}`, addr.String()))
		if err != nil {
			forkLog.Errorf("lazy fetch %s: %v", addr, err)
			return nil, nil, false
		}
		var result struct {
			Exists   bool   `json:"exists"`
			Balance  string `json:"balance"`
			Contract string `json:"contract"`
		}
		if err := utils.JSON.Unmarshal(raw, &result); err != nil {
			forkLog.Errorf("lazy fetch %s: %v", addr, err)
			return nil, nil, false
		}
		if !result.Exists {
			return nil, nil, false
		}

		entry.found = true
		if bal, ok := new(big.Int).SetString(result.Balance, 10); ok && bal.Sign() > 0 {
			entry.bal = bal
		}
		if result.Contract != "" {
			rawAcc, err := hex.DecodeString(result.Contract)
			if err == nil {
				var acc ngtypes.Contract
				if err := rlp.DecodeBytes(rawAcc, &acc); err == nil {
					entry.acc = &acc
				}
			}
		}
		cache[addr] = entry

		kvKeys, codeLen := 0, 0
		if entry.acc != nil {
			kvKeys, codeLen = len(entry.acc.Context.Keys), len(entry.acc.Source)
		}
		forkLog.Warnf("lazily forked %s: balance=%s code=%dB kv=%d", addr, result.Balance, codeLen, kvKeys)
		return entry.acc, entry.bal, entry.found
	})
}

// open (re)opens the working db and rebuilds the State handle around it
func (d *forkChain) open() error {
	db, err := bbolt.Open(d.dbPath, 0o600, nil)
	if err != nil {
		return err
	}
	storage.InitDB(db)
	d.db = db
	// a bare handle over an EXISTING state: no re-init, snapshots unused
	d.state = &ngstate.State{Network: d.network, DB: db, SnapshotManager: &ngstate.SnapshotManager{}}
	return nil
}

// seal applies txs as one instantly-mined dev block through the real
// state-transition path; height/time advance only when it lands
func (d *forkChain) seal(blockTime uint64, txs ...*ngtypes.FullTx) error {
	if blockTime == 0 {
		blockTime = d.blockTime + 1
	}
	err := d.state.Update(func(txn *bbolt.Tx) error {
		return d.state.HandleTxs(txn, blockTime, txs...)
	})
	if err != nil {
		return err
	}
	d.height++
	d.blockTime = blockTime
	return nil
}

// newAddress derives the deterministic signer, registers "@name" and
// prefunds it via a self-signed GenerateTx (the real minting path)
func (d *forkChain) newAddress(name string) (ngtypes.Address, error) {
	if _, taken := d.names[name]; taken {
		return d.names[name], nil
	}
	seed := utils.KeccakSum256([]byte("signer:" + name))
	key, err := ngtypes.NewKeyFromSeed(ngtypes.SchemeSecp256k1, seed)
	if err != nil {
		return ngtypes.Address{}, err
	}
	addr := ngtypes.NewAddress(key)

	mint := ngtypes.NewTx(d.network, ngtypes.GenerateTx, d.height, addr, d.fund, nil, nil, nil)
	if err := mint.Signature(key); err != nil {
		return ngtypes.Address{}, err
	}
	if err := d.seal(0, mint); err != nil {
		return ngtypes.Address{}, err
	}

	d.names[name] = addr
	d.keys[name] = key
	return addr, nil
}

// resolve turns "@name" or a bs58 address into an Address
func (d *forkChain) resolve(who string) (ngtypes.Address, error) {
	if len(who) > 1 && who[0] == '@' {
		addr, ok := d.names[who[1:]]
		if !ok {
			return ngtypes.Address{}, fmt.Errorf("unknown name %q", who)
		}
		return addr, nil
	}
	return ngtypes.NewAddressFromBS58(who)
}

// signerOf finds the key behind "@name" (txs must be signed by a forkChain-
// managed identity)
func (d *forkChain) signerOf(by string) (*ngtypes.PrivateKey, ngtypes.Address, error) {
	if len(by) > 1 && by[0] == '@' {
		by = by[1:]
	}
	key, ok := d.keys[by]
	if !ok {
		return nil, ngtypes.Address{}, fmt.Errorf("no key for %q (dev addresses only)", by)
	}
	return key, d.names[by], nil
}

// ---- rpc plumbing ----

type devHandler func(params []byte) (interface{}, error)

// register binds every dev_* method with shared locking/serialization
func (d *forkChain) register(server *jsonrpc2http.Server) {
	wrap := func(h devHandler) func(*jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		return func(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
			d.mu.Lock()
			defer d.mu.Unlock()

			var params []byte
			if msg.Params != nil {
				params = *msg.Params
			}
			result, err := h(params)
			if err != nil {
				forkLog.Error(err)
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			raw, err := utils.JSON.Marshal(result)
			if err != nil {
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
		}
	}

	server.RegisterJsonRpcHandleFunc("ping", func(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte(`"pong"`))
	})
	server.RegisterJsonRpcHandleFunc("dev_addresses", wrap(d.rpcAddresses))
	server.RegisterJsonRpcHandleFunc("dev_newAddress", wrap(d.rpcNewAddress))
	server.RegisterJsonRpcHandleFunc("dev_deploy", wrap(d.rpcDeploy))
	server.RegisterJsonRpcHandleFunc("dev_call", wrap(d.rpcCall))
	server.RegisterJsonRpcHandleFunc("dev_kv", wrap(d.rpcKV))
	server.RegisterJsonRpcHandleFunc("dev_balance", wrap(d.rpcBalance))
	server.RegisterJsonRpcHandleFunc("dev_mine", wrap(d.rpcMine))
	server.RegisterJsonRpcHandleFunc("dev_setTime", wrap(d.rpcSetTime))
	server.RegisterJsonRpcHandleFunc("dev_snapshot", wrap(d.rpcSnapshot))
	server.RegisterJsonRpcHandleFunc("dev_revert", wrap(d.rpcRevert))
}

// ---- methods ----

type devAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Balance string `json:"balance"`
}

func (d *forkChain) rpcAddresses(_ []byte) (interface{}, error) {
	out := make([]devAddress, 0, len(d.keys))
	for name := range d.keys {
		addr := d.names[name]
		bal, err := d.state.GetTotalBalanceByAddress(addr)
		if err != nil {
			bal = big.NewInt(0)
		}
		out = append(out, devAddress{Name: "@" + name, Address: addr.String(), Balance: bal.String()})
	}
	return out, nil
}

func (d *forkChain) rpcNewAddress(params []byte) (interface{}, error) {
	var p struct {
		Name string `json:"name"`
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name required")
	}
	addr, err := d.newAddress(p.Name)
	if err != nil {
		return nil, err
	}
	return map[string]string{"name": "@" + p.Name, "address": addr.String()}, nil
}

// rpcDeploy commits + activates wasm code under a FRESH named address
// (a contract lives at its deployer's address), through the real
// CommitTx/ActivateTx lifecycle
func (d *forkChain) rpcDeploy(params []byte) (interface{}, error) {
	var p struct {
		Name string `json:"name"`
		Path string `json:"path"` // local wasm file, or
		Wasm string `json:"wasm"` // hex bytecode
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name required")
	}

	var code []byte
	var err error
	switch {
	case p.Path != "":
		code, err = os.ReadFile(p.Path)
	case p.Wasm != "":
		code, err = hex.DecodeString(p.Wasm)
	default:
		err = fmt.Errorf("need path or wasm")
	}
	if err != nil {
		return nil, err
	}

	// the deployer address IS the contract address
	addr, err := d.newAddress(p.Name)
	if err != nil {
		return nil, err
	}
	key := d.keys[p.Name]

	commit := ngtypes.NewTx(d.network, ngtypes.CommitTx, d.height, ngtypes.Address{},
		nil, big.NewInt(1), ngtypes.EncodeCommitCode(code), nil)
	if err := commit.Signature(key); err != nil {
		return nil, err
	}
	if err := d.seal(0, commit); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	activate := ngtypes.NewTx(d.network, ngtypes.ActivateTx, d.height, ngtypes.Address{},
		nil, big.NewInt(1), nil, nil)
	if err := activate.Signature(key); err != nil {
		return nil, err
	}
	if err := d.seal(0, activate); err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}

	// init's receipt (if the contract exports init)
	runs, _ := d.state.GetTxRuns(activate.GetHash())

	return map[string]interface{}{
		"name": "@" + p.Name, "address": addr.String(),
		"codeSize": len(code), "height": d.height, "runs": runs,
	}, nil
}

// rpcCall seals a signed TransactTx to a contract and returns its
// receipt (gas, events, per-run status). dry=true executes and reports
// but ROLLS BACK — state stays untouched (an eth_call analogue). at=N
// sets the sealed block's timestamp (time travel)
func (d *forkChain) rpcCall(params []byte) (interface{}, error) {
	var p struct {
		To     string   `json:"to"`
		By     string   `json:"by"`
		Method string   `json:"method"`
		Args   []string `json:"args"`
		Value  string   `json:"value"` // raw units, decimal
		At     uint64   `json:"at"`
		Dry    bool     `json:"dry"`
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	to, err := d.resolve(p.To)
	if err != nil {
		return nil, err
	}
	key, _, err := d.signerOf(p.By)
	if err != nil {
		return nil, err
	}
	value := big.NewInt(0)
	if p.Value != "" {
		if _, ok := value.SetString(p.Value, 10); !ok {
			return nil, fmt.Errorf("bad value %q", p.Value)
		}
	}
	args, err := packParts(p.Args, d.names)
	if err != nil {
		return nil, err
	}

	tx := ngtypes.NewTx(d.network, ngtypes.TransactTx, d.height, to, value, nil,
		ngtypes.EncodeCallData(p.Method, args), nil)
	if err := tx.Signature(key); err != nil {
		return nil, err
	}

	blockTime := p.At
	if blockTime == 0 {
		blockTime = d.blockTime + 1
	}

	var runs []ngstate.ContractRun
	errRollback := fmt.Errorf("forkChain dry-run rollback")
	err = d.state.Update(func(txn *bbolt.Tx) error {
		if err := d.state.HandleTxs(txn, blockTime, tx); err != nil {
			return err
		}
		runs, _ = ngstate.GetTxRuns(txn, tx.GetHash())
		if p.Dry {
			return errRollback // executed, observed, discarded
		}
		return nil
	})
	if err != nil && err != errRollback {
		return nil, err
	}
	if !p.Dry {
		d.height++
		d.blockTime = blockTime
	}

	return map[string]interface{}{
		"txHash": hex.EncodeToString(tx.GetHash()),
		"height": d.height, "time": blockTime, "dry": p.Dry,
		"runs": runs,
	}, nil
}

// rpcKV reads a contract's storage; the key is the same byte-part DSL
// contract-run uses (@name / str: / u64: / u256: / hex:)
func (d *forkChain) rpcKV(params []byte) (interface{}, error) {
	var p struct {
		Contract string   `json:"contract"`
		Key      []string `json:"key"`
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	addr, err := d.resolve(p.Contract)
	if err != nil {
		return nil, err
	}
	keyBytes, err := packParts(p.Key, d.names)
	if err != nil {
		return nil, err
	}
	acc, err := d.state.GetContract(addr)
	if err != nil {
		return nil, err
	}
	val := acc.Context.Get(string(keyBytes))

	out := map[string]interface{}{"hex": hex.EncodeToString(val), "len": len(val)}
	// convenience decodes for the two standard value widths
	if len(val) > 0 && len(val) <= 8 {
		out["u64"] = leU64(val)
	}
	if len(val) == 32 {
		out["u256"] = new(big.Int).SetBytes(leToBE(val)).String()
	}
	return out, nil
}

func (d *forkChain) rpcBalance(params []byte) (interface{}, error) {
	var p struct {
		Who string `json:"who"`
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	addr, err := d.resolve(p.Who)
	if err != nil {
		return nil, err
	}
	bal, err := d.state.GetTotalBalanceByAddress(addr)
	if err != nil {
		return nil, err
	}
	return map[string]string{"address": addr.String(), "balance": bal.String()}, nil
}

// rpcMine advances empty dev blocks (height/time), for interest accrual
// and deadline debugging
func (d *forkChain) rpcMine(params []byte) (interface{}, error) {
	var p struct {
		Blocks   uint64 `json:"blocks"`
		Interval uint64 `json:"interval"`
	}
	if len(params) > 0 {
		if err := utils.JSON.Unmarshal(params, &p); err != nil {
			return nil, err
		}
	}
	if p.Blocks == 0 {
		p.Blocks = 1
	}
	if p.Interval == 0 {
		p.Interval = 1
	}
	d.height += p.Blocks
	d.blockTime += p.Blocks * p.Interval
	return map[string]uint64{"height": d.height, "time": d.blockTime}, nil
}

func (d *forkChain) rpcSetTime(params []byte) (interface{}, error) {
	var p struct {
		Time uint64 `json:"time"`
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Time <= d.blockTime {
		return nil, fmt.Errorf("time must move forward (now %d)", d.blockTime)
	}
	d.blockTime = p.Time
	return map[string]uint64{"height": d.height, "time": d.blockTime}, nil
}

// rpcSnapshot copies the whole db aside; rpcRevert swaps it back —
// anvil's snapshot/revert, for try-and-rewind debugging loops
func (d *forkChain) rpcSnapshot(_ []byte) (interface{}, error) {
	id := d.nextSnap
	path := fmt.Sprintf("%s.snap%d", d.dbPath, id)
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	err = d.db.View(func(txn *bbolt.Tx) error {
		_, err := txn.WriteTo(f)
		return err
	})
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	d.snapshots[id] = devSnapshot{path: path, height: d.height, blockTime: d.blockTime}
	d.nextSnap++
	return map[string]int{"id": id}, nil
}

func (d *forkChain) rpcRevert(params []byte) (interface{}, error) {
	var p struct {
		ID int `json:"id"`
	}
	if err := utils.JSON.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	snap, ok := d.snapshots[p.ID]
	if !ok {
		return nil, fmt.Errorf("unknown snapshot %d", p.ID)
	}
	if err := d.db.Close(); err != nil {
		return nil, err
	}
	if err := copyFile(snap.path, d.dbPath); err != nil {
		return nil, err
	}
	if err := d.open(); err != nil {
		return nil, err
	}
	d.height, d.blockTime = snap.height, snap.blockTime
	return map[string]uint64{"height": d.height, "time": d.blockTime}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}
