# ngchain Smart Contract Design

The contract engine is a deterministic WebAssembly sandbox built on
[wasman v1](https://github.com/c0mm4nd/wasman), living in the `ngstate`
package. A contract is just the `Contract` bytes of an account; its
persistent storage is the account's `Context` (an on-chain sorted k-v).

## Lifecycle

| step | tx type | effect |
|---|---|---|
| deploy | `AppendTx` / `DeleteTx` | edit the account's `Contract` bytes (only while unlocked) |
| activate | `LockTx` | freeze the bytes, run the optional `init` export once, enable the vm |
| execute | `TransactTx` | when a locked contract account is a participant, its `main` export runs |
| upgrade | `UnlockTx` → edit → `LockTx` | disable the vm, edit, re-activate |

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

Exports a contract may provide:

- `main` — required to react to incoming transact txs
- `init` — optional, runs once on `LockTx`

See `ngstate/wasm_test.go` for hand-assembled example contracts
exercising every module.
