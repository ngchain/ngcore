# ngcore JSON-RPC Reference

Two servers speak JSON-RPC 2.0 over HTTP POST:

- the **node** (`ngcore --rpc-port ...`) — the full chain;
- the **fork tool** (`ngcore fork`) — the instant-seal debugging chain,
  which additionally *consumes* the node's fork-source methods.

All methods are registered in `jsonrpc/regiser_handlers.go`; the fork
tool's in `cmd/ngcore/fork.go`. Both answer `ping` with `"pong"`.

## Encoding conventions

One rule set on every human-facing surface, in params and replies alike:

- **addresses** — the bs58 string (the same text a static wasm import or
  contract source uses);
- **all other raw bytes** (hashes, code, calldata, event data, RLP
  payloads) — **lowercase hex**, never base64;
- **money** — decimal strings of raw units (NG is 18-decimal) in JSON;
  inside the contract ABI it is a fixed 32-byte little-endian u256.

## Node methods

### Chain

| method | what it does |
|---|---|
| `getLatestBlockHeight` / `getLatestBlockHash` / `getLatestBlock` | the current head |
| `getBlockByHeight` / `getBlockByHash` | block lookup |
| `getTxByHash` | tx lookup |
| `getNetwork` | which network the node runs |

### State

| method | what it does |
|---|---|
| `getBalanceByAddress` | total / mature / locked balance of an address |
| `getContract` | the on-chain contract module of an address (hex) |
| `getContractInfo` | the full contract slot (owner, code, context) |
| `getReceipt` | a tx's contract runs — outcome, gas, events (local, derived data) |
| `callContract` | DRY-RUN a contract call against current state — the journal never flushes, a free preview of a transact |

### Fork sources

What `ngcore fork --rpc` pulls from. Ordinary wallets never need these.

| method | what it does |
|---|---|
| `getHead` | light head info: network, height, block hash, timestamp |
| `getAddressState` | ONE address's state — balance + contract (code and storage) as hex RLP; the unit of **lazy** forking |
| `getSheet` | the whole state as one hex-RLP sheet (balances, contracts, key registry); the **eager** fork source |

### Tx composition

Compose unsigned, sign locally, then broadcast — keys never leave the
wallet.

| method | what it does |
|---|---|
| `genTransaction` | unsigned pay/call tx (optional `entry` = export name + hex args) |
| `genCommit` | unsigned commit carrying a whole contract module |
| `genActivate` / `genDeactivate` / `genDestroy` | unsigned lifecycle txs |
| `signTx` | sign an encoded unsigned tx with the node-side key file |
| `sendTx` | broadcast a signed tx |
| `publicKeyToAddress` | derive the bs58 address of a public key |

### Mining & p2p

| method | what it does |
|---|---|
| `getBlockTemplate` / `getWork` / `submitWork` / `submitBlock` | the PoW mining loop |
| `addPeer` / `getPeers` | peer management (`addNode`/`getNodes` aliases) |

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

curl -s localhost:52522 -d '{"jsonrpc":"2.0","id":1,"method":"dev_deploy",
  "params":{"name":"token","path":"ngtoken.wasm"}}'
curl -s localhost:52522 -d '{"jsonrpc":"2.0","id":2,"method":"dev_call",
  "params":{"to":"@token","by":"@dev0","method":"mint",
            "args":["@dev0","u256:1000000000000000000"]}}'
curl -s localhost:52522 -d '{"jsonrpc":"2.0","id":3,"method":"dev_kv",
  "params":{"contract":"@token","key":["str:bal","@dev0"]}}'
```
