package storage

import "github.com/c0mm4nd/dbolt"

var (
	BlockBucketName = []byte("blocks")
	TxBucketName    = []byte("txs")
	// blockTxPrefix = []byte("bt:") // TODO: add block-tx relationship

	// state buckets
	Num2AccBucketName  = []byte("num:acc")
	Addr2BalBucketName = []byte("addr:bal")
	Addr2NumBucketName = []byte("addr:num")
)

var (
	LatestHeightTag = []byte("latest:height")
	LatestHashTag   = []byte("latest:hash")
	OriginHeightTag = []byte("origin:height") // store the origin block
	OriginHashTag   = []byte("origin:hash")
)

func InitDB(db *dbolt.DB) {
	db.Update(func(txn *dbolt.Tx) error {
		_, err := txn.CreateBucketIfNotExists(BlockBucketName)
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
