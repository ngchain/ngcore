# ngcore JSON-RPC Reference

Two servers speak JSON-RPC 2.0 over HTTP POST:

- the **node** (`ngcore --rpc-port ...`) — the full chain;
- the **fork tool** (`ngcore fork`) — the instant-seal debugging chain,
  which additionally *consumes* the node's fork-source methods.

All methods are registered in `jsonrpc/regiser_handlers.go`; the fork
tool's in `cmd/ngcore/fork.go`. Both answer `ping` with `"pong"`.

Method names follow a geth-style namespace convention: `ng_` for chain,
state, tx and mining; `net_` for network info; `admin_` for node/peer
management. `ping` stays bare as a liveness probe. The fork tool keeps
its own `dev_` namespace.

## Encoding conventions

One rule set on every human-facing surface, in params and replies alike:

- **addresses** — the bs58 string (the same text a static wasm import or
  contract source uses);
- **all other raw bytes** (hashes, code, calldata, event data, RLP
  payloads) — **lowercase hex**, never base64;
- **money** — never a float, anywhere: tx-composition params take
  decimal strings of whole NG ("1.5", parsed exactly); balances return
  decimal strings of raw units (NG is 18-decimal); inside the contract
  ABI it is a fixed 32-byte little-endian u256.

## Node methods

### Chain

| method | what it does |
|---|---|
| `ng_getLatestBlockHeight` / `ng_getLatestBlockHash` / `ng_getLatestBlock` | the current head |
| `ng_getBlockByHeight` / `ng_getBlockByHash` | block lookup |
| `ng_getTxByHash` | tx lookup |
| `net_getNetwork` | which network the node runs |

### State

| method | what it does |
|---|---|
| `ng_getBalanceByAddress` | total / mature / locked balance of an address (optional `height`: total as of a past block, archive nodes) |
| `ng_getContractInfo` | the full contract slot (owner, code, context) (optional `height`: the slot as of a past block, archive nodes) |
| `ng_getContractStorage` | ONE contract storage value by raw key (hex); returns hex plus u64/u256 decodes — the targeted read for indexers and wallets (optional `height`: value as of a past block, archive nodes) |

Historical (`height`) reads work by default — archive is the default
startup mode. A node started with `--prune` answers them with an error
rather than a wrong current value. See [archive.md](./archive.md).
| `ng_getReceipt` | a tx's contract runs — outcome, gas, events (local, derived data) |
| `ng_getLogs` | events in a block range (`fromHeight`/`toHeight`, optional `address` emitter and `topic` filters). Internal transactions — contract value transfers — surface automatically as `ng.transfer` logs (emitter = sender, data = to‖value). Archive nodes serve full history; others are bounded to the receipt-retention window |
| `ng_traceTransaction` | a tx's internal call/transfer tree (the "internal transactions"): each run's `trace` is a pre-order list of `call`/`transfer` frames with `depth`, `from`, `to`, `method`, `value`, `input`. Kept even for a reverted run (a re-entrancy-blocked call shows as a frame with `ok:false`), showing where it failed |
| `ng_traceBlock` | the traces of every tx in a block (`height`) that ran a contract |
| `ng_callContract` | DRY-RUN a contract call against current state — the journal never flushes, a free preview of a transact; returns outcome, gas, `events` and the internal `trace` |

### Fork sources

What `ngcore fork --rpc` pulls from. Ordinary wallets never need these.

| method | what it does |
|---|---|
| `ng_getHead` | light head info: network, height, block hash, timestamp |
| `ng_getAddressState` | ONE address's state — balance + contract (code and storage) as hex RLP; the unit of **lazy** forking |
| `ng_getSheet` | the whole state as one hex-RLP sheet (balances, contracts, key registry); the **eager** fork source |

### Tx composition

Compose unsigned, sign locally, then broadcast — keys never leave the
wallet. There is deliberately no `ng_signTx`: the node never sees a
private key. The cli signs the encoded unsigned tx locally and only the
signed bytes reach `ng_sendTx`.

| method | what it does |
|---|---|
| `ng_genTransaction` | unsigned pay/call tx; `value`/`fee` are decimal-NG strings (optional `entry` = export name + hex args) |
| `ng_genCommit` | unsigned commit carrying a whole contract module |
| `ng_genActivate` / `ng_genDeactivate` / `ng_genDestroy` | unsigned lifecycle txs |
| `ng_sendTx` | broadcast a signed tx |
| `ng_suggestFee` | this node's relay fee floor (`minFeePerByte`, decimal raw units); pass a `rawTx` to get the exact `minFee` that tx must carry |
| `ng_publicKeyToAddress` | derive the bs58 address of a public key |

### Node & mempool

Ungated — answerable even while the node is syncing.

| method | what it does |
|---|---|
| `ng_syncing` | whether the node is catching up, and its current tip height |
| `ng_getPendingTxs` | the txs queued in this node's mempool (at most one per sender) |

### Mining & admin

| method | what it does |
|---|---|
| `ng_getWork` / `ng_submitWork` | the PoW mining loop |
| `admin_addPeer` / `admin_getPeers` | peer management |

## Fork-tool methods (`ngcore fork`)

An anvil-style debugging surface; every tx runs the REAL state
transition and seals instantly. Addresses are written as `@name` (a
deterministic dev identity or a deployed contract) or a bs58 address.
Byte parts (`args`, `key`) use the same DSL as `contract-run`:
`@name` (32-byte address) / `str:X` / `u64:N` / `u256:<decimal>` /
`hex:XX`.

| method | params | what it does |
|---|---|---|
| `dev_addresses` | — | the named identities and their balances |
| `dev_newAddress` | `name` | derive + prefund a deterministic identity (keccak("signer:"+name)) |
| `dev_deploy` | `name`, `path`\|`wasm` | real Commit+Activate of a module under a fresh address, registered as `@name` |
| `dev_call` | `to`, `by`, `method`, `args`, `value?`, `at?`, `dry?` | sealed signed transact; returns the receipt (gas, events, per-run status). `dry:true` executes then rolls back; `at:` sets the block time |
| `dev_kv` | `contract`, `key` | read contract storage (hex + u64/u256 decodes) |
| `dev_balance` | `who` | native balance |
| `dev_mine` | `blocks?`, `interval?` | advance empty blocks (height/time) |
| `dev_setTime` | `time` | jump the clock forward |
| `dev_snapshot` | — | whole-state snapshot, returns an id |
| `dev_revert` | `id` | rewind to a snapshot |

Example session:

```sh
ngcore fork --rpc http://node:52521 &   # lazy fork of a live chain

curl -s localhost:52525 -d '{"jsonrpc":"2.0","id":1,"method":"dev_deploy",
  "params":{"name":"token","path":"ngtoken.wasm"}}'
curl -s localhost:52525 -d '{"jsonrpc":"2.0","id":2,"method":"dev_call",
  "params":{"to":"@token","by":"@dev0","method":"mint",
            "args":["@dev0","u256:1000000000000000000"]}}'
curl -s localhost:52525 -d '{"jsonrpc":"2.0","id":3,"method":"dev_kv",
  "params":{"contract":"@token","key":["str:bal","@dev0"]}}'
```
