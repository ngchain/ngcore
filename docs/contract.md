# ngchain Smart Contract Design

The contract engine is a deterministic WebAssembly sandbox built on
[wasman](https://github.com/c0mm4nd/wasman), living in the `ngstate`
package.

A contract lives on chain as a **compiled WebAssembly module** in the
address's `Contract` slot: any language that targets wasm (Rust,
AssemblyScript, TinyGo, C) produces a contract, and the module is
validated (full parse + static type check) before it can be committed or
activated, so every active contract is guaranteed loadable. Identical
modules deployed by many addresses are stored ONCE, content-addressed by
their keccak hash. The contract's persistent storage is the address's
`Context` (an on-chain sorted k-v). Wat text appears throughout this doc
as the readable notation for wasm — on chain lives the binary.

## Lifecycle

| step | tx type | effect |
|---|---|---|
| deploy / edit | `CommitTx` | replace the whole module (only while unlocked); the first commit opens the slot |
| activate | `ActivateTx` | validate + freeze the module, run the optional `init` export once, enable the vm |
| execute | `TransactTx` | paying an address with an ACTIVE contract runs the export named in the calldata (`main` fallback) |
| upgrade | `DeactivateTx` → `CommitTx` → `ActivateTx` | disable the vm, replace the module, re-activate |

A `CommitTx`'s extra carries the WHOLE module — a full snapshot, like a
git commit stores a blob, not a diff. Diffing made sense when contracts
were hand-written text; compiled wasm relayouts entirely on any change,
so a patch would be as large as the module. The wire encoding is a
one-byte format tag plus the bytes, deflate-compressed when that
shrinks them; decode caps the inflated size against decompression
bombs, and the state layer enforces `MaxContractSourceSize`.

Tooling: the `ng_genCommit` RPC composes the unsigned commit tx from the
module bytes (`ngcore cli commit --file contract.wasm`); `ng_genActivate` /
`ng_genDeactivate` / `ng_genDestroy` cover the rest of the lifecycle;
`ng_getContractInfo` reads the current on-chain slot (owner, module,
context) and `ng_getContractStorage` reads one storage value by key. Sign
the unsigned tx locally, then broadcast with `ng_sendTx`. See
[rpc.md](./rpc.md) for the full method reference.

The lock flag is stored in the address's context under the reserved key
`_active`. Keys prefixed with `_` are system-reserved: reads through the
kv host module see nothing (a probe is harmless), but a `set`/`del` on a
reserved key TRAPS the call — silently dropping the write would turn an
authoring bug into hours of debugging.

## Determinism & safety

- one fresh `Module` + `Linker` + `Instance` per call — no state leaks between txs
- `DisableFloatPoint: true` — no platform-dependent float results
- `Recover: true` — a contract panic can never kill the node
- `CallDepthLimit: 512`
- gas: `SimpleTollStation` with a fixed budget of 2^24 toll per call;
  every instruction costs 1, and host operations charge tiered extras
  (kv.set 1000 + 10/byte, kv.del 500, kv reads 100, coin.transfer 2000,
  log.emit 500 + 5/byte, each cross-contract call 2000, code
  introspection 100 + len/8) so state writes cost orders of magnitude
  more than arithmetic; overflowing aborts the call
- linear memory is capped at 64 pages (4 MiB) per instance, declared AND
  grown: toll bounds instructions, but memory.grow allocates 64 KiB of
  host memory for ~1 toll — the cap closes that gap
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
         ; attributed to the executing address, dropped on failed runs.
         ; topics under the reserved "ng." namespace are refused (return 0)
address: get_size() -> i32          ; address length (32)
         get_host(ptr) -> i32       ; writes the EXECUTING address
         get_caller(ptr) -> i32     ; msg.From address (zero addr at top)
contract: call(addr_ptr, args_ptr, args_len) -> i32
         ; invoke the contract at a RUNTIME address on its OWN state (1|0)
         is_active(addr_ptr) -> i32      ; an active contract lives there?
         get_code_size(addr_ptr) -> i32
         get_code(addr_ptr, ptr) -> i32  ; the on-chain code bytes
         code_hash(addr_ptr, ptr) -> i32 ; 32-byte keccak of the code
crypto:  keccak256(ptr, size, out) -> i32
         verify(scheme, pk_ptr, pk_len, hash_ptr, sig_ptr, sig_len) -> i32
         addr_of(scheme, pk_ptr, pk_len, out) -> i32
coin:    get_balance(addr_ptr, ptr) -> i32   ; fixed 32-byte LE amount
         transfer(to_ptr, value_ptr) -> i32  ; value: 32-byte LE amount
         ; money crosses the ABI as a FIXED 32-byte little-endian value —
         ; the u256/token wire format, full 256-bit range (NG is 18-decimal).
         ; each transfer auto-emits an "ng.transfer" log (data = to‖value),
         ; so internal transfers are queryable via ng_getLogs
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
u256:    mul_div(dst, a, b, c)      ; floor(a*b/c), full 512-bit product
         isqrt(dst, a)              ; floor(sqrt(a))
         ; wide-integer extension: values are 16/32-byte little-endian
         ; limbs in linear memory passed by pointer — deterministic
         ; 256-bit token math without floats (evm division conventions)
tx:      get_hash_size() -> i32      get_hash(ptr) -> i32
         get_network() -> i32        get_height() -> i64
         get_timestamp() -> i64      ; enclosing block time (unix s)
         get_paid(ptr) -> i32
         ; msg.value: what this tx pays to the EXECUTING address
         ; (zero unless it is the To address), fixed 32-byte LE amount
         get_from(ptr) -> i32       ; the tx's From address
         get_to(ptr) -> i32         ; the tx's To address
         get_fee_size() -> i32       get_fee(ptr) -> i32
         get_extra_size() -> i32     get_extra(ptr) -> i32
         ; the ARGS of the decoded CallData (the method already consumed)
```

## Calling a contract

A transact tx paying an address with an ACTIVE contract runs it. The tx
extra is an RLP `CallData{method, args}` — ngcore dispatches by the
export NAME (wasm modules already carry named exports, so there is no
eth-style 4-byte selector):

```
extra = rlp([method, args])
```

The runtime runs the export named `method` (when it is a zero-arg export;
the reserved `init` entry excluded), with `tx.get_extra` serving `args`.
An empty/`main` method, an absent export, or an extra that is not a
CallData falls back to `main`, which receives its args (the whole extra
when the payload was not a CallData). `ng_callContract` (rpc dry-run)
resolves the same way, so read-only methods like a `balance_of` export
are directly callable off-chain.

## Module dependencies

Contracts compose by CALLING one another. A contract imports another
LOCKED contract's exports directly by its deployer bs58 ADDRESS:

```wat
;; call another contract's export; it runs on ITS OWN state (tokens,
;; pools, any shared ledger)
(import "<id>" "transfer" (func $transfer (param i64 i64) (result i32)))
```

`<id>` is the deployer's bs58 address — the address IS the namespace:
it anchors WHO published the code you call, like a Go module path, with
no name registry to squat, no numbers to race for, and no prefix to pick.

One encoding convention across every surface: wherever a HUMAN writes an
address (imports, rpc params, scenarios, contract source) it is this
same bs58 text; the 32-byte machine form is what flows at runtime
(calldata, `get_caller`, the `contract.call` argument). SDKs convert at
compile time — ngwasm's `const fn addr("bs58...")` decodes the literal
into its 32 bytes in the compiler, so the string never crosses the ABI
and a typo is a compile error.

Every dependency is a SERVICE: each call switches the execution frame to
the dependency's address — its kv/coin effects act on ITS OWN state,
which is exactly how a token keeps one ledger shared by all callers.
`address.get_caller` writes the invoking contract's address (msg.from)
for authorization; `address.get_host` the executing one. Within one
transaction execution, a contract that is still executing cannot be
re-entered (calls after it returned are fine); service exports use scalar
(i32/i64) params and returns — byte payloads (u256 amounts, strings)
cross through the env transfer slots: the caller stages bytes with
buf_set before the call, the callee reads them with buf_get and returns
results the same way.

There is no library form running on the CALLER's state (there was once a
`contract/` namespace for it). It was the only primitive that let
external code touch your storage — a sharp footgun, and the sole source
of a "whose state does this run on?" choice. Removing it makes
composition uniform: a dependency ALWAYS runs on its own state. Shared
pure code is reused by inlining it (identical bytecode is deduplicated on
chain by hash).

Shared rules:

- dependencies are declared STATICALLY by the wat import section, so
  the chain extracts them at lock time — no runtime analysis needed
- a dependency must be active before its dependents can activate; this
  ordering makes the dependency graph a DAG by construction
- every dependee carries a reference count: while referenced it can be
  neither deactivated nor destroyed, so linked code never changes under
  a dependent. Deactivating the dependent releases its references
- the ledger lives in the reserved context keys `_deps` (dependent's
  list) and `_refs` (dependee's counter), invisible to contracts
- the whole call tree shares ONE gas budget

For calling a contract chosen at RUNTIME (not fixed at lock time), a
contract uses the dynamic `contract.call(addr, calldata)` host function
instead of a static import — same service semantics, address resolved on
the fly, no reference pinning. This is the common path for tokens and
pools addressed by a variable.

## Immutability & upgrades

A referenced contract is frozen: while `_refs > 0` it can be neither
deactivated nor destroyed. This is a **feature, not a limitation**.
When B depends on A, B's author audited *that* code; if A could
rewrite itself, B would live under the shadow of a dependency turning
malicious (imagine A is a token that one day rewrites `transfer` to
pay itself). Immutability is the guarantee B receives, not a penalty A
suffers — it is what makes cross-contract composition trustable at all.

So the chain has no in-place upgrade, by design. Two honest models
cover every need:

**Publish a new version (the default).** Once referenced a contract is
immutable: you do not edit the v1 others depend on, you deploy v2 at a
new address and dependents migrate by re-declaring the v2 address and
re-activating. Old versions never disappear; migration is a social,
opt-in act — exactly how classical DeFi legos are built on immutable
contracts. Fragmentation is the price of not betraying anyone's audit.

**Delegate to a proxy (opt-in mutability).** If you want upgradeability,
build it in wat: deploy a thin proxy P that forwards to an
implementation address stored in its own kv, and have dependents point
at P. Upgrading means P's owner repoints the kv slot. The trust model
downgrades honestly — a dependent of P trusts P's owner not to swap in
malice — but that is the dependent's *informed* choice, not something
the protocol imposed. The primitives are already here: `kv` for the
pointer, `address.get_caller` to gate who may repoint, and a dynamic
`contract.call` to forward.

```wat
;; a minimal upgradeable proxy: owner repoints "impl", everyone else's
;; calls forward to whatever address that slot holds
(module
  (import "address" "get_caller" (func $caller (param i32) (result i32)))
  (import "kv" "get" (func $get (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "tx" "get_extra" (func $args (param i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "implownr")     ;; keys: "impl", "ownr"
  ;; on "set_impl": if caller == stored owner, store the new impl addr
  ;; (a real proxy would then forward main/other entries to the stored
  ;;  impl address via a dynamic call; omitted here for brevity)
  (func (export "set_impl")
    (drop (call $caller (i32.const 64)))       ;; caller -> 64
    (drop (call $get (i32.const 4) (i32.const 4) (i32.const 96))) ;; owner -> 96
    ;; compare 32 bytes at 64 vs 96; if equal, accept the new impl from args
    (drop (call $args (i32.const 128)))        ;; new impl addr -> 128
    (drop (call $set (i32.const 0) (i32.const 4) (i32.const 128) (i32.const 32)))))
```

Rule of thumb: **immutable by default, upgradeable by explicit proxy.**
The protocol gives you the trustable primitive; upgrade policy is
yours to compose, and its risks are yours to own.

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
Query with the `ng_getReceipt` rpc ({hash}) — it also reports the tx's
block and confirmations; `ng_callContract` previews the events a real tx
would emit. Note: a node which fast-synced (snapshot mode) has no
receipts for txs below its checkpoint, since it never executed them.

See `ngstate/wasm_test.go` for more contracts exercising every module.
