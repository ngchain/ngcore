# ngchain Smart Contract Design

The contract engine is a deterministic WebAssembly sandbox built on
[wasman v1](https://github.com/c0mm4nd/wasman), living in the `ngstate`
package.

A contract lives on chain as **WebAssembly text (wat)** in the account's
`Contract` field — human-readable, diff-able and editable in place by
patch (edit) txs, like a script. It is compiled to binary by the
deterministic `wat.Compile` only when the account gets locked (and on
each execution); an account whose text does not compile cannot be
locked, so every active contract is guaranteed valid. The contract's
persistent storage is the account's `Context` (an on-chain sorted k-v).

## Lifecycle

| step | tx type | effect |
|---|---|---|
| deploy / edit | `CommitTx` | apply a patch (hunks) onto the contract text atomically (only while unlocked) |
| activate | `ActivateTx` | compile-check + freeze the text, run the optional `init` export once, enable the vm |
| execute | `TransactTx` | when an active contract account is a participant, its `main` export runs |
| upgrade | `DeactivateTx` → `CommitTx` → `ActivateTx` | disable the vm, patch the text, re-activate |

An `CommitTx` carries an encoded `CommitExtra` patch: each hunk replaces
bytes at an offset of the ORIGINAL text; hunks are sorted, must not
overlap, and a stale patch fails whole. A small source change therefore
costs a patch proportional to the change, not to the contract size.

The wire encoding minimizes the tx further:

- **shape** — when the removed content outweighs one hash, the patch
  pins the original text with its keccak-256 (32 bytes flat) and drops
  the `Del` bytes entirely (hunks carry just `DelLen`); tiny patches
  keep the cheaper content shape. `NewCommitExtra` picks automatically.
- **compression** — `Encode` deflates the payload when that shrinks it
  (large deploys of repetitive wat text compress well; tiny patches
  stay raw). `DecodeCommitExtra` caps the decompressed size at
  `TxMaxExtraSize` against zip bombs.

Tooling: the `genContractUpdate` RPC (and the
`ngcore cli contract-update --file new.wat` subcommand) diffs
the on-chain text against the new text server-side (line LCS + byte
shrinking) and returns the unsigned minimal-patch CommitTx; `genEdit`
accepts explicit hunks; `getContract` reads the current text. Sign and
broadcast with the existing `signTx` / `sendTx` methods.

The lock flag is stored in the account context under the reserved key
`_active`. Keys prefixed with `_` are system-reserved and invisible to
contracts through the kv host module.

## Determinism & safety

- one fresh `Module` + `Linker` + `Instance` per call — no state leaks between txs
- `DisableFloatPoint: true` — no platform-dependent float results
- `Recover: true` — a contract panic can never kill the node
- `CallDepthLimit: 512`
- gas: `SimpleTollStation` with a fixed budget of 2^24 toll per call;
  every instruction costs 1, and host operations charge tiered extras
  (kv.set 1000 + 10/byte, kv.del 500, kv reads 100, coin.transfer 2000,
  log.emit 500 + 5/byte, each service call 2000) so state writes cost
  orders of magnitude more than arithmetic; overflowing aborts the call
- all host functions bounds-check every pointer/length against the
  instance's linear memory

## Atomicity (journal)

Contract writes (balance transfers, kv changes) go into an in-memory
journal with read-your-writes overlay. The journal flushes into the db
transaction only when the entry call returns successfully. A trap, gas
overflow or failed host call discards the whole journal — but never
fails the enclosing tx: the base value transfer stands, and every node
reaches the same state.

## Host ABI

Import modules available to contracts (wasm32 types only; `*_size`
funcs return the byte length to allocate before the paired getter
writes into `ptr`):

```
log:     debug(ptr, size)            error(ptr, size)
         emit(tptr, tlen, dptr, dlen) -> i32
         ; records an event (topic + data) into the tx's LOCAL receipt;
         ; attributed to the executing address, dropped on failed runs
account: get_size() -> i32          ; address length (32)
         get_host(ptr) -> i32       ; writes the EXECUTING address
         get_caller(ptr) -> i32     ; msg.From address (zero addr at top)
         get_contract_size(addr_ptr) -> i32
         get_contract(addr_ptr, ptr) -> i32
         is_active(addr_ptr) -> i32
coin:    get_balance_size(addr_ptr) -> i32
         get_balance(addr_ptr, ptr) -> i32   ; big-endian bytes
         transfer(to_ptr, value: i64) -> i32
kv:      get_size(kptr, klen) -> i32
         get(kptr, klen, vptr) -> i32
         set(kptr, klen, vptr, vlen) -> i32
         del(kptr, klen) -> i32
         count(pptr, plen) -> i32     ; prefix enumeration over the
         key_size_at(pptr, plen, i) -> i32   ; sorted, non-reserved keys
         key_at(pptr, plen, i, out) -> i32
env:     get_gas() -> i64            ; remaining toll of the call tree
         buf_set(slot, ptr, len) -> i32   ; cross-frame transfer slots
         buf_size(slot) -> i32            ; (8 x 4KB): byte payloads
         buf_get(slot, ptr) -> i32        ; crossing service boundaries
u128/u256: add/sub/mul/div_u/div_s/rem_u/rem_s(dst, a, b)
         and/or/xor(dst, a, b)   not(dst, a)
         shl/shr_u/shr_s(dst, a, bits)
         cmp_u/cmp_s(a, b) -> i32   iszero(a) -> i32
         ; wide-integer extension: values are 16/32-byte little-endian
         ; limbs in linear memory passed by pointer — deterministic
         ; 256-bit token math without floats (evm division conventions)
tx:      get_hash_size() -> i32      get_hash(ptr) -> i32
         get_network() -> i32        get_height() -> i64
         get_timestamp() -> i64      ; enclosing block time (unix s)
         get_paid_size() -> i32      get_paid(ptr) -> i32
         ; msg.value: what this tx pays to the EXECUTING address
         ; (zero unless it is the To address), big-endian big.Int bytes
         get_sender(ptr) -> i32     ; the tx from's address
         get_to(ptr) -> i32         ; the tx's To address
         get_fee_size() -> i32       get_fee(ptr) -> i32
         get_extra_size() -> i32     get_extra(ptr) -> i32
         ; the ARGS part of the calldata (see the selector convention)
```

## Calling a contract

A transact tx paying an address with an ACTIVE contract runs it. The
tx extra addresses the entry eth-style:

```
extra = keccak256(entry name)[:4] ‖ args
```

The runtime matches the 4-byte selector against the contract's
zero-arg exports (sorted by name; the reserved `init` entry excluded)
and runs the match with `tx.get_extra` serving `args`. An empty extra,
a short extra or an unmatched selector falls back to `main`, which —
like eth's fallback function — receives the WHOLE extra as its args.
`callContract` (rpc dry-run) resolves the same way, so read-only
methods like a `balance_of` export are directly callable off-chain.

## Module dependencies

Contracts compose like code modules with TWO dependency semantics,
picked per import by its namespace:

```wat
;; library: code runs on the CALLER's state (math, curves, algorithms)
(import "contract/<id>" "double" (func $double (param i64) (result i64)))

;; service: code runs on the DEPENDENCY's own state (tokens, pools,
;; any shared ledger)
(import "service/<id>" "transfer" (func $transfer (param i64 i64) (result i32)))
```

`<id>` is the deployer's bs58 address — the address IS the namespace:
it anchors WHO published the code you link against, like a Go module
path, with no name registry to squat or numbers to race for.

Shared rules:

- dependencies are declared STATICALLY by the wat import section, so
  the chain extracts them at lock time — no runtime analysis needed
- a dependency must be locked (active) before its dependents can lock;
  this ordering makes the dependency graph a DAG by construction
- every dependee carries a reference count: while referenced it can be
  neither unlocked nor destroyed, so linked code never changes under a
  dependent. Deactivating the dependent releases its references
- the ledger lives in the reserved context keys `_deps` (dependent's
  list) and `_refs` (dependee's counter), invisible to contracts
- the whole call tree shares ONE gas budget

Library semantics (`contract/<addr>`): the dependency's code links directly
and runs with the caller's host modules — its kv/coin effects act on
the calling address. A library contributes code, not state.

Service semantics (`service/<addr>`): each call switches the execution frame
to the dependency's account — its kv/coin effects act on ITS OWN state,
which is exactly how a token keeps one ledger shared by all callers.
`account.get_caller` writes the invoking contract's address
(msg.from) for authorization; `account.get_host` the executing one.
Within one transaction execution, a contract that is still executing
cannot be re-entered (calls after it returned are fine); service
exports use scalar (i32/i64) params and returns — byte payloads
(u256 amounts, strings) cross through the env transfer slots: the
caller stages bytes with buf_set before the call, the callee reads
them with buf_get and returns results the same way.

Exports a contract may provide:

- `main` — required to react to incoming transact txs
- `init` — optional, runs once on `ActivateTx`

Example — a complete on-chain contract:

```wat
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
```

## Receipts & events (non-consensus)

Every contract run a tx triggers lands in a LOCAL receipt (tx hash ->
runs with outcome, error, gas and emitted events). Receipts never enter
block hashes: each node derives them deterministically by executing the
chain, and a reorg replay regenerates them for the winning branch.
Query with the `getReceipt` rpc ({hash}) — it also reports the tx's
block and confirmations; `callContract` previews the events a real tx
would emit. Note: a node which fast-synced (snapshot mode) has no
receipts for txs below its checkpoint, since it never executed them.

See `ngstate/wasm_test.go` for more contracts exercising every module.
