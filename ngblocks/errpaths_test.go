package ngblocks

import (
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// viewErr runs fn inside a read-only txn and returns whatever error fn
// produces. Because the buckets come from a read-only transaction, any
// Put/Delete inside fn fails with bbolt.ErrTxNotWritable, which lets the
// write-error branches of the store be exercised without corrupting the db.
func viewErr(t *testing.T, db *bbolt.DB, fn func(blockBucket, txBucket *bbolt.Bucket) error) error {
	t.Helper()

	var out error
	if err := db.View(func(txn *bbolt.Tx) error {
		out = fn(txn.Bucket(storage.BlockBucketName), txn.Bucket(storage.TxBucketName))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	return out
}

// blockKey occupies key inside the block bucket with a nested sub-bucket. A
// later Bucket.Put on that same key then fails with ErrIncompatibleValue,
// which lets targeted (not just first) write-error branches be reached inside
// an otherwise writable transaction.
func blockKey(t *testing.T, db *bbolt.DB, key []byte) {
	t.Helper()

	if err := db.Update(func(txn *bbolt.Tx) error {
		b := txn.Bucket(storage.BlockBucketName)
		// the key may already hold a regular value (e.g. a latest tag); drop
		// it first so CreateBucket can claim the same key as a sub-bucket
		if err := b.Delete(key); err != nil {
			return err
		}
		_, err := b.CreateBucket(key)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// putBlock's second Put (the height->hash index) fails when the height key is
// occupied by a sub-bucket, while the first Put (hash->block) still succeeds.
func TestPutBlockHeightIndexWriteError(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	b1 := buildBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)

	blockKey(t, db, utils.PackUint64LE(b1.GetHeight()))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err == nil {
			t.Fatal("putBlock height index must fail when the key is a sub-bucket")
		}
		return nil
	})
}

// putLatestTags' second Put (the latest-hash tag) fails when that tag key is a
// sub-bucket, exercising the branch after the latest-height tag was written.
func TestPutLatestTagsHashWriteError(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)

	blockKey(t, db, storage.LatestHashTag)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err == nil {
			t.Fatal("putLatestTags hash tag must fail when the key is a sub-bucket")
		}
		return nil
	})
}

// InitFromCheckpoint surfaces the very first Put error (hash->block) when the
// checkpoint hash key is occupied by a sub-bucket.
func TestInitFromCheckpointWriteError(t *testing.T) {
	db := newDB(t)
	store := Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)

	blockKey(t, db, b1.GetHash())

	if err := store.InitFromCheckpoint(b1); err == nil {
		t.Fatal("InitFromCheckpoint must fail when the hash key is a sub-bucket")
	}
}

// PutNewBlock on a read-only txn: linkage checks read fine, then putBlock's
// first Put fails, so PutNewBlock returns the write error.
func TestPutNewBlockWriteErrors(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	b2 := buildBlock(t, b1, miner)

	// commit b1 so b2's prev-hash check passes on the read-only txn
	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return PutNewBlock(blockBucket, txBucket, b1)
	})

	if err := viewErr(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return PutNewBlock(blockBucket, txBucket, b2)
	}); err == nil {
		t.Fatal("PutNewBlock on a read-only txn must fail")
	}
}

// putBlock/putTxs/putLatestTags each surface their write errors; running a
// no-tx-check ForcePutNewBlock on a read-only txn exercises putBlock and the
// wrapping in put_force.go.
func TestForcePutNewBlockWriteErrors(t *testing.T) {
	db := newDB(t)
	store := Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)

	// commit b1 so its prev (genesis) exists for the read-only replay
	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return store.ForcePutNewBlock(blockBucket, txBucket, b1)
	})

	// height 1 already exists -> delTxs runs first and fails on the read txn
	alt1 := buildBlock(t, genesis, newKey(t))
	if err := viewErr(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return store.ForcePutNewBlock(blockBucket, txBucket, alt1)
	}); err == nil {
		t.Fatal("ForcePutNewBlock delTxs on a read-only txn must fail")
	}

	// a brand-new height (no existing block) skips delTxs and fails on putBlock
	b2 := buildBlock(t, b1, miner)
	if err := viewErr(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return store.ForcePutNewBlock(blockBucket, txBucket, b2)
	}); err == nil {
		t.Fatal("ForcePutNewBlock putBlock on a read-only txn must fail")
	}
}

// ForcePutNewBlock must surface a corrupted existing block at the target
// height (GetBlockByHeight decode error) rather than proceeding.
func TestForcePutNewBlockExistingBroken(t *testing.T) {
	db := newDB(t)
	store := Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	alt1 := buildBlock(t, genesis, newKey(t))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		// point height 1 at a hash whose payload is undecodable
		if err := blockBucket.Put(utils.PackUint64LE(1), b1.GetHash()); err != nil {
			return err
		}
		return blockBucket.Put(b1.GetHash(), []byte{0xff})
	})

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		err := store.ForcePutNewBlock(blockBucket, txBucket, alt1)
		if err == nil {
			t.Fatal("ForcePutNewBlock must fail when the existing block is corrupted")
		}
		return nil
	})
}

// GetBlockByHeight: the height key resolves to a hash, but no block is stored
// under that hash -> the second lookup returns ErrKeyNotFound.
func TestGetBlockByHeightDanglingHash(t *testing.T) {
	db := newDB(t)

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		return blockBucket.Put(utils.PackUint64LE(3), make([]byte, 32))
	})

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		if _, err := GetBlockByHeight(blockBucket, 3); err == nil {
			t.Fatal("dangling height must fail with not-found")
		}
		return nil
	})
}

// SwitchToBranch: a valid branch on a read-only txn reaches the delTxs write
// and fails there, covering the switch's mutation error paths.
func TestSwitchToBranchWriteErrors(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	minerA := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, minerA)
	a2 := buildBlock(t, b1, minerA)
	c2 := buildBlock(t, b1, newKey(t))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err != nil {
			return err
		}
		return PutNewBlock(blockBucket, txBucket, a2)
	})

	if err := viewErr(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{c2})
	}); err == nil {
		t.Fatal("SwitchToBranch on a read-only txn must fail")
	}
}

// SwitchToBranch: a canonical height between the fork point and the tip is
// broken (dangling hash) -> the replace loop returns the wrapped decode error.
func TestSwitchToBranchBrokenCanonical(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	minerA := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, minerA)
	a2 := buildBlock(t, b1, minerA)
	c2 := buildBlock(t, b1, newKey(t))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err != nil {
			return err
		}
		return PutNewBlock(blockBucket, txBucket, a2)
	})

	// corrupt the canonical block at height 2 so the switch loop can't read it
	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		return blockBucket.Put(a2.GetHash(), []byte{0xff})
	})

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{c2}); err == nil {
			t.Fatal("SwitchToBranch must fail on a broken canonical chain")
		}
		return nil
	})
}

// SwitchToBranch: the ancestor exists but the latest-height tag is missing, so
// GetLatestHeight errors before the mutation loop.
func TestSwitchToBranchMissingLatestHeight(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	minerA := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, minerA)
	c2 := buildBlock(t, b1, newKey(t))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return PutNewBlock(blockBucket, txBucket, b1)
	})

	// drop the latest-height tag while leaving the canonical ancestor intact
	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		return blockBucket.Delete(storage.LatestHeightTag)
	})

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{c2}); err == nil {
			t.Fatal("SwitchToBranch must fail when the latest height tag is gone")
		}
		return nil
	})
}

// SwitchToBranch extending the tip (branch starts at latest+1) skips the
// replace/delTxs loop entirely and goes straight to the connect loop, whose
// putBlock then fails on the read-only txn.
func TestSwitchToBranchExtendTipWriteError(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	minerA := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, minerA)
	b2 := buildBlock(t, b1, minerA)

	// canonical tip is b1 (height 1); the branch [b2] starts at height 2 == tip+1
	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return PutNewBlock(blockBucket, txBucket, b1)
	})

	if err := viewErr(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{b2})
	}); err == nil {
		t.Fatal("SwitchToBranch connect loop on a read-only txn must fail")
	}
}

// PutSideBlock on a read-only txn fails on its first Put.
func TestPutSideBlockWriteError(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	side := buildBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)

	if err := viewErr(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		return PutSideBlock(blockBucket, side)
	}); err == nil {
		t.Fatal("PutSideBlock on a read-only txn must fail")
	}
}

// PruneSideBlocks on a read-only txn fails on the first Delete of a victim.
func TestPruneSideBlocksWriteError(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	side := buildBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)

	// commit one prunable side block (height 1) so a victim exists
	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		return PutSideBlock(blockBucket, side)
	})

	if err := viewErr(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		_, err := PruneSideBlocks(blockBucket, 2)
		return err
	}); err == nil {
		t.Fatal("PruneSideBlocks on a read-only txn must fail")
	}
}
