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
| deploy / edit | `EditTx` | apply a patch (hunks) onto the contract text atomically (only while unlocked) |
| activate | `LockTx` | compile-check + freeze the text, run the optional `init` export once, enable the vm |
| execute | `TransactTx` | when a locked contract account is a participant, its `main` export runs |
| upgrade | `UnlockTx` → `EditTx` → `LockTx` | disable the vm, patch the text, re-activate |

An `EditTx` carries an encoded `EditExtra` patch: each hunk replaces
bytes at an offset of the ORIGINAL text; hunks are sorted, must not
overlap, and a stale patch fails whole. A small source change therefore
costs a patch proportional to the change, not to the contract size.

The wire encoding minimizes the tx further:

- **shape** — when the removed content outweighs one hash, the patch
  pins the original text with its sha3-256 (32 bytes flat) and drops
  the `Del` bytes entirely (hunks carry just `DelLen`); tiny patches
  keep the cheaper content shape. `NewEditExtra` picks automatically.
- **compression** — `Encode` deflates the payload when that shrinks it
  (large deploys of repetitive wat text compress well; tiny patches
  stay raw). `DecodeEditExtra` caps the decompressed size at
  `TxMaxExtraSize` against zip bombs.

Tooling: the `genContractUpdate` RPC (and the
`ngcore cli contract-update --num N --file new.wat` subcommand) diffs
the on-chain text against the new text server-side (line LCS + byte
shrinking) and returns the unsigned minimal-patch EditTx; `genEdit`
accepts explicit hunks; `getContract` reads the current text. Sign and
broadcast with the existing `signTx` / `sendTx` methods.

The lock flag is stored in the account context under the reserved key
`_locked`. Keys prefixed with `_` are system-reserved and invisible to
contracts through the kv host module.

## Determinism & safety

- one fresh `Module` + `Linker` + `Instance` per call — no state leaks between txs
- `DisableFloatPoint: true` — no platform-dependent float results
- `Recover: true` — a contract panic can never kill the node
- `CallDepthLimit: 512`
- gas: `SimpleTollStation` with a fixed budget of 2^24 instructions per
  call; overflowing aborts the call
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
account: get_host() -> i64
         get_owner_size() -> i32     get_owner(num: i64, ptr) -> i32
         get_contract_size(num: i64) -> i32
         get_contract(num: i64, ptr) -> i32
         is_locked(num: i64) -> i32
coin:    get_balance_size(num: i64) -> i32
         get_balance(num: i64, ptr) -> i32   ; big-endian bytes
         transfer(to: i64, value: i64) -> i32
kv:      get_size(kptr, klen) -> i32
         get(kptr, klen, vptr) -> i32
         set(kptr, klen, vptr, vlen) -> i32
         del(kptr, klen) -> i32
tx:      get_hash_size() -> i32      get_hash(ptr) -> i32
         get_network() -> i32        get_height() -> i64
         get_convener() -> i64
         get_participants_count() -> i32
         get_participant_size() -> i32
         get_participant(i, ptr) -> i32
         get_value_size(i) -> i32    get_value(i, ptr) -> i32
         get_fee_size() -> i32       get_fee(ptr) -> i32
         get_extra_size() -> i32     get_extra(ptr) -> i32
```

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

`<id>` addresses the dependency in either form:

- `<deployerBS58>.<name>` — the RECOMMENDED handle: a lock tx with a
  non-empty extra registers `<owner-address>.<name>` for the contract
  ([a-z0-9_-], max 32; unique per deployer, released on destroy). The
  deployer's address in the identifier anchors WHO published the code
  you link against, like a Go module path
- `<num>` — the raw hosting account number (low-level form)

Shared rules:

- dependencies are declared STATICALLY by the wat import section, so
  the chain extracts them at lock time — no runtime analysis needed
- a dependency must be locked (active) before its dependents can lock;
  this ordering makes the dependency graph a DAG by construction
- every dependee carries a reference count: while referenced it can be
  neither unlocked nor destroyed, so linked code never changes under a
  dependent. Unlocking the dependent releases its references
- the ledger lives in the reserved context keys `_deps` (dependent's
  list) and `_refs` (dependee's counter), invisible to contracts
- the whole call tree shares ONE gas budget

Library semantics (`contract/N`): the dependency's code links directly
and runs with the caller's host modules — its kv/coin effects act on
the calling account. A library contributes code, not state.

Service semantics (`service/N`): each call switches the execution frame
to the dependency's account — its kv/coin effects act on ITS OWN state,
which is exactly how a token keeps one ledger shared by all callers.
`account.get_caller` returns the invoking contract's num (msg.sender)
for authorization; `account.get_host` returns the executing account.
Within one transaction execution, a contract that is still executing
cannot be re-entered (calls after it returned are fine); service
exports use scalar (i32/i64) params and returns.

Exports a contract may provide:

- `main` — required to react to incoming transact txs
- `init` — optional, runs once on `LockTx`

Example — a complete on-chain contract:

```wat
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
```

See `ngstate/wasm_test.go` for more contracts exercising every module.
