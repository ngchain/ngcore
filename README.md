<h1> <img src="./resources/ng_16x16.png" >core</h1>

ngcore is the Go implementation of **ngchain**: a proof-of-work chain
built around two nouns and six verbs, designed by subtraction.

## The whole model in one paragraph

An **Address** (32-byte keccak hash of a public key) is the identity,
the balance holder and the namespace — nothing is registered, any key
spends directly. A **Contract** is the code slot an address may open
under its own namespace: a compiled WebAssembly module (any language
that targets wasm), committed whole like a git blob, frozen and
executed while active. That's the entire ontology.

## Six tx verbs

| Verb | Effect |
|---|---|
| `Generate` | the mining reward |
| `Transact` | pay an address; runs its active contract (the call routes to the export named in the payload, `main` is the fallback) |
| `Commit` | replace the own contract module (deflate snapshot); the first commit opens the slot |
| `Activate` | validate + freeze the module, turn the vm on (runs `init` once) |
| `Deactivate` | turn the vm off, reopen the module for commits |
| `Destroy` | remove the own slot entirely |

Every tx is `{Network, Type, Height, To, Value, Fee, Extra, Sign}` —
the sender is derived from the signature envelope, fees are burned.

## Signatures: a menu, not a monoculture

Keys derive from a 32-byte seed under a per-key scheme:

| Scheme | Envelope | Role |
|---|---|---|
| secp256k1 (default) | **67 B** (key recovery, eth parity) | classical efficiency |
| FN-DSA-512 | 700 B compact | the small post-quantum option |
| ML-DSA-44 | 2.5 KB compact | the finalized FIPS 204 pick |
| SLH-DSA-128s | 7.9 KB | hash-based, assumption-minimal, SNARK-friendliest |

Post-quantum keys reveal their public key once (auto-registered on
chain); later spends carry only `From ‖ sig`. Quantum migration is a
per-key wallet choice, not a hard fork.

**Witness separation**: txids hash the unsigned tx; a header-level
witness root commits the signature envelopes. Settled history can
later drop or replace its signatures (pruning, LaBRADOR-style
aggregate proofs) without touching a single txid.

## Contracts

On-chain code is a compiled wasm module, validated at commit and
activation and deduplicated by code hash. Contracts compose by CALLING
one another: a contract imports another by its `<deployer address>` (or
calls one at runtime via `contract.call`), and the callee always runs on
its OWN state (tokens, pools) with re-entry guarded and
statically-declared dependees reference-pinned. Execution is journaled
(all-or-nothing), gas-tiered (state writes cost orders of magnitude more
than arithmetic), and receipts with events stay local — never consensus
data. Money crosses the contract ABI as fixed 32-byte little-endian
u256, so an 18-decimal native coin and token amounts share one format.

Tooling: `ngcore contract-run` executes JSON test scenarios against the
real VM (batch/CI), and `ngcore fork --rpc <node>` forks a RUNNING chain
lazily — state fetched per address on first touch — into an instant-seal
local copy for interactive debugging (anvil-style, with receipts,
dry-run, time travel and snapshot/revert).

## Docs

- [docs/contract.md](docs/contract.md) — the contract model: lifecycle,
  host ABI, dispatch, dependencies, upgrades
- [docs/implementation.md](docs/implementation.md) — the VM internals:
  module cache, gas, wideint, metered JIT, fork tooling
- [docs/rpc.md](docs/rpc.md) — the JSON-RPC method reference (node +
  fork tool)
- [docs/positioning.md](docs/positioning.md) — the design rationale

## Consensus

- CPU PoW (AstroBWT), 1s target blocks, cumulative-work fork choice
- atomic reorgs: chain switch + full state replay in ONE db txn
- rolling finality every 10 blocks; orphan pool; side-block pruning
- consensus caps: 512 txs / 8 MiB per block; witness root enforced
- keccak-256 for every chain hash; genesis carries no premine

## Quick start

```bash
go build -o ngcore ./cmd/ngcore

# a throwaway local chain
./ngcore --zeronet --in-mem

# wallet (keys stay local; only signed txs travel)
./ngcore cli key --new --scheme secp256k1
./ngcore cli status
./ngcore cli balance
./ngcore cli send --to <bs58> --value 1.5 --fee 0.0001
./ngcore cli commit --file contract.wasm --fee 0.0001   # first commit deploys
./ngcore cli activate --fee 0.0001
./ngcore cli call --contract <bs58> --entry balance_of

# fork a running chain for contract debugging (lazy, anvil-style)
./ngcore fork --rpc http://127.0.0.1:52521
```

## Status

Experimental. Consensus formats change freely between versions and
dev chains restart; MAINNET parameters are intentionally undefined.
