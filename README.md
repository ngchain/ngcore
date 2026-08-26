<h1> <img src="./resources/ng_16x16.png" >core</h1>

ngcore is the Go implementation of **ngchain**: a post-quantum,
account-based proof-of-work chain with a private-by-default mempool and a
programmable, WebAssembly contract layer — designed by subtraction.

> 📄 **The full protocol specification lives at
> [paper.ngchain.org](https://paper.ngchain.org).** This README is a short
> orientation and a build/run guide; the paper is authoritative for every
> consensus rule, byte layout and constant.

## The model in one paragraph

An **Address** (32 bytes, the BLAKE3 hash of a public key under its scheme)
is the identity, the balance holder and the namespace — nothing is
registered, any key spends directly. A **Contract** is the compiled
WebAssembly module an address may open under its own namespace. Three
transaction operations act on this: `Generate` (the mining reward),
`Transact` (pay an address and run its contract), and `Deploy` (create,
upgrade UUPS-style, or destroy the sender's own contract). That is the whole
ontology.

## What's distinctive

- **Post-quantum by construction** — a BLAKE3-256 content hash and four
  account signature schemes: secp256k1 plus the post-quantum FN-DSA, ML-DSA
  and SLH-DSA. Keys reveal their public key once (auto-registered), later
  spends carry only `From ‖ sig`.
- **A private mempool, by default** — every effect tx is the reveal half of a
  mandatory commit–reveal, so contents stay hidden from miners until the
  committing block is sealed. Both halves are relayed by the node, so a
  wallet sends one call and may go offline.
- **UUPS contracts** — deploy / upgrade / destroy are one operation; upgrade
  logic lives in the contract (`ng:upgrade`), and a contract with no such hook
  is immutable.
- **Native account abstraction** — an account installs an `ng:validate` hook
  the protocol runs on every one of its txs (spend limits, freezes,
  allow-lists), on top of the native signature.
- **Reserved `ng:` hooks** — `ng:main` / `ng:init` / `ng:upgrade` /
  `ng:validate` are protocol-bound; every other export is an ordinary method.

See the paper for consensus (astrobwt PoW, cumulative-work fork choice,
GHOST uncles), the two block roots (content + witness), and the full contract
VM and host ABI.

## Quick start

```bash
go build -o ngcore ./cmd/ngcore

# a throwaway local chain
./ngcore --zeronet --in-mem

# wallet — keys stay local; only signed txs travel
./ngcore cli key --new --scheme secp256k1
./ngcore cli status
./ngcore cli balance

# send / deploy are private by default: the node relays the commit-reveal,
# so these return immediately and land over the next few blocks
./ngcore cli send   --to <bs58> --value 1.5 --fee 0.0001
./ngcore cli deploy --file contract.wasm --fee 0.0001   # first deploy goes live
./ngcore cli call   --contract <bs58> --entry balance_of

# fork a running chain for contract debugging (lazy, anvil-style)
./ngcore fork --rpc http://127.0.0.1:52521   # serves its own rpc on :52525
```

## Docs

- **[paper.ngchain.org](https://paper.ngchain.org)** — the protocol
  specification (consensus, state, transactions, the private mempool, the
  contract VM, account abstraction, and a security/threat-model section)
- [docs/rpc.md](docs/rpc.md) — the JSON-RPC method reference (node + fork
  tool); the source of truth is the registry in `jsonrpc/`

## Status

Experimental. Consensus formats change freely between versions and dev
chains restart; MAINNET parameters are intentionally undefined.
