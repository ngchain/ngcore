package storage

import "go.etcd.io/bbolt"

var (
	BlockBucketName = []byte("blocks")
	TxBucketName    = []byte("txs")

	// TxBlockPrefix keys (inside the tx bucket) map a tx hash to the
	// hash of the block containing it
	TxBlockPrefix = []byte("blk:")

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
