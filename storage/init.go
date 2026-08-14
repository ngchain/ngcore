package storage

import "go.etcd.io/bbolt"

var (
	BlockBucketName = []byte("blocks")
	TxBucketName    = []byte("txs")

	// TxBlockPrefix keys (inside the tx bucket) map a tx hash to the
	// hash of the block containing it
	TxBlockPrefix = []byte("blk:")

	// state buckets
	Num2AccBucketName  = []byte("num:acc")
	Addr2BalBucketName = []byte("addr:bal")
	Addr2NumBucketName = []byte("addr:num")

	// SnapshotBucketName persists the checkpoint state sheets, so the
	// mature-balance lookups survive restarts
	SnapshotBucketName = []byte("snapshot")

	// ContractNameBucketName maps deployer-address + name to the account
	// num hosting the contract, backing the addr.name import form
	ContractNameBucketName = []byte("contract:names")

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
		_, err := txn.CreateBucketIfNotExists(BlockBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(ReceiptBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(ContractNameBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(SnapshotBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(TxBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(Num2AccBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(Addr2BalBucketName)
		if err != nil {
			return err
		}

		_, err = txn.CreateBucketIfNotExists(Addr2NumBucketName)
		if err != nil {
			return err
		}

		return nil
	})
}
