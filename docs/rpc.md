# ngcore JSON-RPC Reference

> This is a practical method reference. For the **protocol** — the private
> mempool, tx model, contract VM and consensus these methods expose — see the
> specification at [paper.ngchain.org](https://paper.ngchain.org). The source
> of truth for the method set is the registry in `jsonrpc/regiser_handlers.go`.

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

The node also serves **WebSocket** at `/ws` on the same port: every method
above is callable over it, plus push subscriptions (below).

### Subscriptions (WebSocket)

`ng_subscribe` opens a push stream and returns a subscription id;
`ng_unsubscribe` (with `{ "id": ... }`) closes it. Events arrive as
`ng_subscription` notifications carrying `{ subscription, result }`.

| `type` | result pushed |
|---|---|
| `newHeads` | `{height, hash, prevHash, timestamp}` on every new tip |
| `logs` | each matching log (same shape as `ng_getLogs`); optional `address` / `topic` filter |
| `pendingTxs` | `{hash}` when a tx enters this node's mempool |

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
| `ng_getTransactionsByAddress` | an address's tx history (sent or received), height-ordered; `fromHeight`/`toHeight`/`limit` |
| `ng_getBlockTransactionCount` / `ng_getTransactionByBlockAndIndex` | block tx navigation by `height`\|`hash` (+ `index`) |
| `ng_getDifficulty` | current difficulty and block reward |
| `net_getNetwork` | which network the node runs |

### State

| method | what it does |
|---|---|
| `ng_getBalanceByAddress` | total / mature / locked balance of an address (optional `height`: total as of a past block, archive nodes) |
| `ng_getContractInfo` | the full contract slot (owner, code, context) (optional `height`: the slot as of a past block, archive nodes) |
| `ng_getContractStorage` | ONE contract storage value by raw key (hex); returns hex plus u64/u256 decodes — the targeted read for indexers and wallets (optional `height`: value as of a past block, archive nodes) |
| `ng_getContractExports` | a contract's exported functions (its ABI), marking those a transact tx can call (optional `height`) |
| `ng_getReceipt` | a tx's contract runs — outcome, gas, events (local, derived data) |
| `ng_getLogs` | events in a block range (`fromHeight`/`toHeight`, optional `address` emitter and `topic` filters). Internal transactions — contract value transfers — surface automatically as `ng.transfer` logs (emitter = sender, data = to‖value). Archive nodes serve full history; others are bounded to the receipt-retention window |
| `ng_traceTransaction` | a tx's internal call/transfer tree (the "internal transactions"): each run's `trace` is a pre-order list of `call`/`transfer` frames with `depth`, `from`, `to`, `method`, `value`, `input`. Kept even for a reverted run (a re-entrancy-blocked call shows as a frame with `ok:false`), showing where it failed |
| `ng_traceBlock` | the traces of every tx in a block (`height`) that ran a contract |
| `ng_callContract` | DRY-RUN a contract call — the journal never flushes; returns outcome, gas, `events` and the internal `trace`. Optional `height`: simulate against the state reconstructed at a past block (isolated scratch db, no archive needed) |

Historical (`height`) reads come in two flavours. The per-address reads
(`ng_getBalanceByAddress`, `ng_getContractInfo`, `ng_getContractStorage`,
`ng_getAddressState`) resolve from the changeset index and need **archive**
(the default startup mode; `--prune` disables them). The whole-state reads
(`ng_getSheet`, `ng_callContract`) instead **reconstruct** the state at the
height in an isolated scratch db — they work on any node that still has the
blocks (archive or not), at the cost of a replay that grows with distance
from the tip. See [archive.md](./archive.md).

### Fork sources

What `ngcore fork --rpc` pulls from. Ordinary wallets never need these.

| method | what it does |
|---|---|
| `ng_getHead` | light head info: network, height, block hash, timestamp |
| `ng_getAddressState` | ONE address's state — balance + contract (code and storage) as hex RLP; the unit of **lazy** forking (optional `height`: as of a past block, archive nodes) |
| `ng_getSheet` | the whole state as one hex-RLP sheet (balances, contracts, key registry); the **eager** fork source. Optional `height`: the full state as of a past block, reconstructed in an isolated scratch db (any node with the blocks; slower far from the tip) |

### Tx composition

Compose unsigned, sign locally, then broadcast — keys never leave the
wallet. There is deliberately no `ng_signTx`: the node never sees a
private key. The cli signs the encoded unsigned tx locally and only the
signed bytes reach `ng_sendTx`.

Effect txs (`Transact`/`Deploy`) are **private by default**: each is the reveal
half of a mandatory commit–reveal, so the wallet submits a blind commitment plus
a signed reveal and the node relays both across the window until they land. The
cli's `send`/`deploy`/`destroy` drive this in one call.

| method | what it does |
|---|---|
| `ng_genTransaction` | unsigned pay/call tx; `value`/`fee` are decimal-NG strings (optional `entry` = export name + hex args) |
| `ng_genDeploy` | unsigned deploy tx carrying a whole contract module; empty module = destroy (UUPS, authorized by the contract's own `ng:upgrade`). `ng_genCommit` is a compatibility alias |
| `ng_sendTx` | broadcast a signed tx (an effect tx is admissible only as the reveal of a prior on-chain commitment) |
| `ng_sendCommitment` | pool + gossip a signed blind commitment |
| `ng_sendReveal` | hand a signed reveal to this node to relay (retargeted per block, no re-signing) until it lands |
| `ng_sendPrivateTx` | one-call fire-and-forget: relay a commitment and its reveal together |
| `ng_suggestFee` | this node's relay fee floor (`minFeePerByte`, decimal raw units); pass a `rawTx` to get the exact `minFee` that tx must carry |
| `ng_publicKeyToAddress` | derive the bs58 address of a public key |

### Node & mempool

Ungated — answerable even while the node is syncing.

| method | what it does |
|---|---|
| `ng_syncing` | whether the node is catching up, and its current tip height |
| `ng_getPendingTxs` | the txs queued in this node's mempool (at most one per sender) |
| `net_nodeInfo` | node self-description: peer id, wired protocol, network, version, peer count, listen addrs |
| `net_peerCount` | number of known peers |

### Mining & admin

| method | what it does |
|---|---|
| `ng_getWork` / `ng_submitWork` | the PoW mining loop |
| `admin_addPeer` / `admin_removePeer` / `admin_getPeers` | peer management |

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
| `dev_deploy` | `name`, `path`\|`wasm` | deploy a module under a fresh address (goes live at once), registered as `@name` |
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
