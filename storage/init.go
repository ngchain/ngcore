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
	Addr2BalBucketName = []byte("addr:bal")

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
	db.Update(func(txn *bbolt.Tx) error {
		for _, name := range [][]byte{
			BlockBucketName, TxBucketName,
			ContractBucketName, Addr2BalBucketName,
			SnapshotBucketName, ReceiptBucketName,
		} {
			if _, err := txn.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}

		return nil
	})
}
