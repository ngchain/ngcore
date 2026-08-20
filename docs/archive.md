# Archive: historical state

An archive node can answer "what was this address's balance / contract
storage at block height N", not just at the current tip. ngcore does this
the Erigon/reth way — **flat current state + per-block changesets + an
inverted index** — so a historical read is an indexed lookup, NOT a
replay.

## Model

The current ("plain") state stays where it always was: one value per
address in `addr:bal`, `addr:contract`, `addr:key`. Archive adds, per
applied block, the **pre-image** of every mutated address:

| bucket | key | value | order |
|---|---|---|---|
| `cs:bal` | `heightLE ‖ addr` | tagged old balance | block-major (unwind) |
| `cs:contract` | `heightLE ‖ addr` | tagged old slot blob | block-major (unwind) |
| `cs:key` | `heightLE ‖ addr` | tag (registry is append-only) | block-major (unwind) |
| `hist:bal` | `addr ‖ heightLE` | ∅ | addr-major (point query) |
| `hist:contract` | `addr ‖ heightLE` | ∅ | addr-major (point query) |

The two orders mirror Erigon's `ChangeSet` (block-keyed) + `History`
(address-keyed) split: block-major for reorg unwind, address-major for
point queries. The pre-image tag distinguishes "absent" from a concrete
value, which balance-0 alone cannot.

Because ngcore already writes the **whole value per address** (the entire
contract `Context` is one blob in the slot) and every mutation funnels
through four helpers (`setBalance`, `setContract`, `delContract`,
`registerPubKey` in `ngstate/state_helper.go`), the changeset grain is
per-address and there are exactly four capture points. Finer per-kv
storage diffs would require splitting `Context` into its own bucket — a
later refinement.

## Read algorithm (no replay)

```
balanceAt(addr, N):
  M = smallest height > N at which addr changed        # hist:bal cursor seek
  if M exists:  return preimage(cs:bal[M‖addr])         # value overwritten at M = value @N
  else:         return current plain balance             # never changed after N
```

`contractAt` / storage-at is identical, then decode the slot's `Context`.
See `ngstate/changeset.go`.

## Capture path

`Upgrade` sets a per-block recorder (`state.cs`) for the height being
applied, when `state.Archive` is on. The write helpers capture each
mutated address's pre-image once per height (first write wins = the value
at height−1) before overwriting. On a non-archive node the recorder is
nil and nothing is captured — no storage cost, no behavior change.

Content-addressed code is refcount-GC'd; on an archive node
`releaseCode` keeps the bytes even at refcount 0, so a historical slot can
still resolve its module.

## Enablement

Archive is the **default startup mode** (`state.Archive`, set on by the
`InitStateFrom*` constructors so capture begins at genesis). `--prune`
opts out: a prune node keeps only the current state, answers historical
(`height`) RPC reads with an error rather than a wrong current value, and
its reorgs fall back to replay.

A `height` query is bounded to what the archive can answer truthfully:
above the chain tip is rejected (no future state), and below the tip the
resolver requires the changesets to actually cover `(height, tip]` —
checked via `changesetCovers(height+1)`. A height with no recorded
history (a snapshot-started node below its checkpoint, or a pre-archive
db not yet backfilled) is refused rather than answered with current
state. This coverage check, not block-origin, is the honest floor.

Upgrading a **pre-archive** db in place is handled on startup:
`State.BackfillArchive` detects "blocks present but no changesets" on a
genesis-origin node and does a one-time replay to rebuild the history
(a no-op on fresh, already-covered, snapshot-started, or prune nodes).

## Reorg unwind

A reorg reverts state to the fork point by applying the changesets
backward (`State.UnwindToTxn`), then rolls the new branch forward
(`State.ApplyBlocksTxn`) — O(reorg depth) instead of replaying the whole
chain from genesis. `switchToBranchTxn` (`blockchain/fork.go`) attempts
the unwind first and falls back to `RebuildFromBlockStoreTxn` only when
it cannot (archive off, or the changesets do not reach the fork point on
a snapshot-started node). `changesetCovers` gates this: every applied
archive height carries at least the coinbase balance change, so the
presence of the fork height's changeset implies the whole range above it.

## Cost notes

Steady-state capture is cheap (one pre-image per changed address per
block) and consensus-neutral: `DumpSheetTxn` excludes the changeset
buckets, so archive and prune nodes produce byte-identical state sheets
and snapshot hashes; capture is a write-only side effect that never feeds
block hashing, VM toll/gas, or validation.

The expensive path is the **full-chain replay in one txn**
(`RebuildFromBlockStoreTxn`): bbolt buffers all dirty pages until commit,
and archive roughly doubles the write volume. Default archive nodes avoid
it — reorgs use unwind (O(reorg depth)). It is reached only by
`BackfillArchive` (a one-time upgrade of a large pre-archive db) and by
the reorg replay fallback (prune nodes, which don't capture, so no
archive amplification). Backfill logs a warning before it runs.

## Roadmap

- **P1 (done)** — buckets, recorder + capture, `GetBalanceByAddressAt` /
  `GetContractAt`, optional `height` on `ng_getBalanceByAddress` /
  `ng_getContractInfo` / `ng_getContractStorage`.
- **P2 (done)** — reorg by **unwind** instead of replay-from-genesis;
  archive default-on from genesis.
- **P3 (done)** — `--prune` opt-out flag, a coverage-based guard on
  `height` reads (reject uncovered heights and above tip), and a one-pass
  replay **backfill** (`BackfillArchive`) on startup for in-place
  upgrades of pre-archive dbs.
- **Optional** — a shallow-window retention for prune nodes: capture a
  bounded changeset window so their reorgs still *unwind* instead of
  replay, without keeping full history. Not implemented — prune nodes
  currently replay on reorg (the pre-archive behavior), which is fine for
  the shallow, infrequent reorgs of a PoW chain.
