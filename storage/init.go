package storage

import "go.etcd.io/bbolt"

var (
	BlockBucketName = []byte("blocks")
	TxBucketName    = []byte("txs")

	// TxBlockPrefix keys (inside the tx bucket) map a tx hash to the
	// hash of the block containing it
	TxBlockPrefix = []byte("blk:")

	// AddrTxPrefix keys (inside the tx bucket) index the txs an address
	// touches: atx: ‖ addr(32) ‖ heightLE(8) ‖ txHash(32) -> nil. A prefix
	// cursor over an address yields its txs in height order (account history)
	AddrTxPrefix = []byte("atx:")

	// state buckets
	// ContractBucketName maps an address to its contract slot
	// (opened by the first commit — the address IS the namespace)
	ContractBucketName = []byte("addr:contract")
	// CodeBucketName is the content-addressed code store: codeHash ->
	// refcount ‖ wasm. Identical modules deployed by many addresses
	// share one physical copy, released when the last slot drops it
	CodeBucketName     = []byte("code")
	Addr2BalBucketName = []byte("addr:bal")

	// KeyRegistryBucketName maps an address to its revealed public
	// key: written by the first full-envelope tx of the address, it
	// lets every later tx use the compact (key-less) envelope
	KeyRegistryBucketName = []byte("addr:key")

	// SnapshotBucketName persists the checkpoint state sheets, so the
	// mature-balance lookups survive restarts
	SnapshotBucketName = []byte("snapshot")

	// ReceiptBucketName holds the LOCAL (non-consensus) execution
	// receipts: tx hash -> contract runs with their events. Every node
	// rebuilds them deterministically by executing the chain
	ReceiptBucketName = []byte("receipts")

	// --- archive: changesets (block-major) + history indices (addr-major) ---
	// Erigon/reth-style historical state: the current plain state stays in
	// the buckets above; these record, per applied block, the PRE-IMAGE of
	// every mutated address so any past height can be resolved by index
	// lookup (no replay), and reorgs can unwind instead of replaying from
	// genesis. Written only when the node runs with archive enabled.

	// BalChangeSetBucketName: key = heightLE(8) ‖ addr(32) -> tagged old
	// balance (absent-tombstone or 0x01 ‖ bytes). Block-major, for unwind
	BalChangeSetBucketName = []byte("cs:bal")
	// ContractChangeSetBucketName: key = heightLE(8) ‖ addr(32) -> tagged
	// old contract-slot blob. Block-major, for unwind
	ContractChangeSetBucketName = []byte("cs:contract")
	// KeyChangeSetBucketName: key = heightLE(8) ‖ addr(32) -> tag. The key
	// registry is append-only (first reveal), so the pre-image is always
	// absent; recorded so a reorg can drop the reveal
	KeyChangeSetBucketName = []byte("cs:key")

	// BalHistBucketName: key = addr(32) ‖ heightLE(8) -> ∅. Addr-major
	// inverted index; bbolt keeps keys sorted, so a prefix cursor yields
	// the change-heights in order for point queries
	BalHistBucketName = []byte("hist:bal")
	// ContractHistBucketName: key = addr(32) ‖ heightLE(8) -> ∅
	ContractHistBucketName = []byte("hist:contract")
)

var (
	LatestHeightTag = []byte("latest:height")
	LatestHashTag   = []byte("latest:hash")
	OriginHeightTag = []byte("origin:height") // store the origin block
	OriginHashTag   = []byte("origin:hash")
)

func InitDB(db *bbolt.DB) {
	err := db.Update(func(txn *bbolt.Tx) error {
		for _, name := range [][]byte{
			BlockBucketName, TxBucketName,
			ContractBucketName, CodeBucketName, Addr2BalBucketName,
			KeyRegistryBucketName,
			SnapshotBucketName, ReceiptBucketName,
			BalChangeSetBucketName, ContractChangeSetBucketName, KeyChangeSetBucketName,
			BalHistBucketName, ContractHistBucketName,
		} {
			if _, err := txn.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}

		return nil
	})
	// bucket creation failing leaves the db unusable — every later
	// txn.Bucket() returns nil and panics far from here. Fail loudly at
	// the source instead of swallowing the error
	if err != nil {
		log.Panicf("failed to initialize db buckets: %v", err)
	}
}
