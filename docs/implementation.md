# ngcore Implementation — the contract VM

How ngcore actually runs a contract. This is the implementation companion
to [contract.md](./contract.md) (the on-chain contract model) and
[positioning.md](./positioning.md) (why the design is shaped this way).
Everything here lives in the `ngstate` package unless noted.

## The VM at a glance

A contract runs inside a `VM` (`ngstate/wasm.go`), a deterministic
WebAssembly sandbox built on [wasman](https://github.com/c0mm4nd/wasman).
A **fresh VM is built for every single call** — nothing in it survives
across txs — which is what keeps execution deterministic and side-effect
free until commit.

```
TransactTx ─▶ NewVM ─▶ load module ─▶ bind host imports ─▶ link deps
                                                              │
                          EntryFor(extra) ─▶ Run(entry) ─▶ journal.flush
```

State changes are staged in a **journal** and flushed into the bbolt txn
only when the call fully succeeds; a trap, a gas overflow, or a missing
entry leaves chain state untouched.

## Loading a module

`NewVM` resolves the module through a small pipeline:

1. `LoadContractWasm` checks the `\0asm` magic and validates the binary
   (parse + full static validation). The source is UNTRUSTED, so a loader
   panic is recovered into an error — a malicious commit can never crash
   the node.
2. **Module cache** (`moduleCache`, keyed by `sha256(code)`): the decoded
   + validated template is memoized, so a contract is parsed and
   validated *once*, not on every call. Callers get a **shallow copy**
   bound to their own per-call `ModuleConfig`. This is safe because
   wasman's `Instantiate` builds fresh, instance-owned functions and
   index spaces from the read-only decoded sections and only writes back
   to the Module pointer it is handed — the per-call copy, never the
   shared template. Concurrency-safe (verified under `-race`).

The per-call `ModuleConfig` fixes the execution contract:

| field | value | why |
|---|---|---|
| `DisableFloatPoint` | true | floats are not deterministic across platforms |
| `Recover` | true | a contract panic must never kill the node |
| `CallDepthLimit` | 512 | bound the wasm call stack |
| `EnableWideInt` | true | the `u128`/`u256` host modules (256-bit money) |
| `TollStation` | SimpleTollStation(1<<24) | the gas meter (below) |
| `EnableJIT` | true | native execution, gas-identical (below) |

## Host ABI

Contracts import ngcore's host functions by module name. Values that
cross a contract boundary pass through `env` buf slots, since instances
do not share linear memory.

| module | functions |
|---|---|
| `kv` | get / get_size / set / del — the account's on-chain k-v (`Context`) |
| `tx` | get_from / get_to / get_paid / get_extra / get_height / get_timestamp |
| `address` | get_host / get_caller / is_active |
| `coin` | transfer / get_balance — native NG |
| `crypto` | keccak256 / verify / addr_of |
| `env` | buf_set / buf_get — cross-frame byte payloads; get_gas |
| `log` | error / emit (events) |
| `u128` / `u256` | 128/256-bit add/sub/mul/div/rem/cmp/shift + `mul_div`, `isqrt` |
| `service/<addr>`, `contract/<addr>` | cross-contract calls (see Dependencies) |

## Call dispatch — by export name

A call payload is an RLP `CallData{Method, Args}` (`ngtypes/tx_extra.go`).
ngcore dispatches to the export **named** by `Method`; there is no
eth-style 4-byte selector (a wasm module already has named exports, so
the selector — and its collision class — is pure overhead).

`EntryFor` (outer tx) and `resolveDynEntry` (dynamic `service.call`) share
one rule:

- a non-empty `Method` naming a **zero-arg export** (the reserved `init`
  excluded) runs that export, with `Args` served through `tx.get_extra`;
- an empty/`main` method, an absent export, or an extra that is not a
  CallData at all falls back to the default `main` entry, which sees its
  args (the whole extra when the payload was not a CallData).

## Gas — the toll model

Determinism requires a hard, identical bound on work per call. A
`TollStation` charges a **flat toll per wasm instruction**, capped at
`vmMaxToll = 1<<24`; exceeding it traps and rolls the call back. Host
operations that are much heavier than arithmetic add a surcharge on top:

| op | extra toll |
|---|---|
| kv set | 1000 + 10/byte |
| kv del | 500 |
| kv read | 100 |
| coin transfer | 2000 |
| event | 500 + 5/byte |
| service call | 2000 |

The block layer enforces a shared budget by **pre-burning** the station
(`LimitToll`): the per-call default budget starts charged by the amount
the block can no longer afford, so a drained block clamps and then skips
runs. `GasUsed` reports a run's own consumption with that preburn
subtracted.

## 256-bit integers

`EnableWideInt` exposes `u128`/`u256` host modules operating on 32-byte
little-endian operands in linear memory (EVM division conventions:
divide-by-zero yields zero). Token amounts are exact `U256`, so an
18-decimal asset is precise. The expensive operations — `mul_div` (full
512-bit intermediate), `div`, `isqrt` — run as native Go, which is both
fast and deterministic (integer results are exact and platform
independent). Contract SDKs may fall back to a pure-wasm `U256` when the
host module is absent; the two are bit-for-bit equivalent.

## Native execution — the metered JIT

`EnableJIT` compiles eligible function bodies to native code. The
critical property for a consensus chain is that the baseline JIT is
**inline-metered**: it charges the *same* per-op toll as the interpreter
and traps at the same cap. Because `SimpleTollStation` prices every
opcode uniformly, gas is byte-identical whether the JIT runs or not —
verified across every scenario, on vs off. A node that JIT-compiles
(arm64) and one that runs the metered interpreter (other targets)
therefore agree on both result and gas. Requires wasman ≥ v1.7.1.

## Dependencies — libraries and services

A contract may import another *locked* contract's exports, declared
statically in its wasm import section and extracted at lock time
(`ngstate/deps.go`). Two namespaces, two semantics:

- **`contract/<addr>` — library.** The dependency's code links directly
  and runs on the CALLER's state: it contributes code, not its own
  ledger. Delegate semantics.
- **`service/<addr>` — service.** Calls run on the dependency's OWN state
  (tokens, pools, any shared ledger); `address.get_caller` exposes the
  invoking contract. This is how tokens and AMMs compose.

Dependencies instantiate before their dependents (a DAG by activation
order), the chain **reference-counts** dependees (a depended-on contract
can be neither deactivated nor destroyed until released), and the link
depth is bounded. `service.call` additionally allows **dynamic** dispatch
by a runtime address, so one compiled module (an AMM pair, a router)
works against any target resolved at call time.

## Determinism guarantees

Everything above composes into a single promise — identical result and
identical gas on every node:

- no floats (`DisableFloatPoint`);
- flat, capped, pre-burnable per-instruction gas;
- integer-only 256-bit math (exact, platform-independent);
- JIT gas byte-identical to the interpreter;
- a fresh, journalled VM per call — no cross-call state, no partial
  writes on failure;
- untrusted bytecode validated before execution, panics recovered.

## Testing

Contracts are exercised against the real VM through the
`ngcore contract-run <scenario.json>` subcommand
(`cmd/ngcore/contracttest.go`): it deploys wasm on an in-memory genesis
fork, runs a sequence of signed calls, prints per-step toll, and checks
the resulting on-chain state (u64 or u256 values). External contract
projects (ngwasm, ngswap, nglend) test their Rust through this binary
with no Go in their own trees.
