package blockchain

import (
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// internalDB opens a fresh bbolt db with all buckets, for testing the
// unexported fork helpers directly.
func internalDB(t *testing.T) *bbolt.DB {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "fork.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)

	return db
}

// sealBlock crafts a hash-stable sealed ZERONET block on the parent hash at
// the given height. The fork helpers only inspect linkage/height/difficulty,
// not pow validity, so a trivial seal is enough.
func sealBlock(t *testing.T, height uint64, prevHash []byte, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16
	diff := big.NewInt(1)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, prevHash, diff)

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	if err := block.ToSealed(utils.PackUint64LE(0)); err != nil {
		t.Fatal(err)
	}

	return block
}

func mustKey(t *testing.T) *ngtypes.PrivateKey {
	t.Helper()
	k, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// collectBranch must reject a branch whose fork point sits BELOW a non-genesis
// origin (checkpoint) block: the walk hits height originHeight+1 with a prev
// hash that is not the origin, so it cannot attach to anything stored.
func TestCollectBranchBelowOrigin(t *testing.T) {
	db := internalDB(t)
	miner := mustKey(t)

	// pretend the origin is a checkpoint at height 10
	const originHeight = uint64(10)
	origin := sealBlock(t, originHeight, make([]byte, 32), miner)

	// a side block at originHeight+1 whose prev is NOT the origin hash: it
	// descends from a block below the origin that this node does not store
	belowParent := make([]byte, 32)
	belowParent[0] = 0xef // != origin hash
	side := sealBlock(t, originHeight+1, belowParent, miner)

	if err := db.Update(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		// set the origin tags to the checkpoint block
		if err := bb.Put(storage.OriginHeightTag, utils.PackUint64LE(originHeight)); err != nil {
			return err
		}
		if err := bb.Put(storage.OriginHashTag, origin.GetHash()); err != nil {
			return err
		}
		// store the origin canonically so the side block is not canonical
		if err := bb.Put(utils.PackUint64LE(originHeight), origin.GetHash()); err != nil {
			return err
		}
		return ngblocks.PutSideBlock(bb, side)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		_, err := collectBranch(bb, side)
		if !errors.Is(err, ErrReorgBelowOrigin) {
			t.Fatalf("collectBranch = %v, want ErrReorgBelowOrigin", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// collectBranch accepts a branch that forks EXACTLY at a non-genesis origin:
// the block at originHeight+1 builds directly on the origin hash.
func TestCollectBranchForksAtOrigin(t *testing.T) {
	db := internalDB(t)
	miner := mustKey(t)

	const originHeight = uint64(10)
	origin := sealBlock(t, originHeight, make([]byte, 32), miner)
	side := sealBlock(t, originHeight+1, origin.GetHash(), miner)

	if err := db.Update(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if err := bb.Put(storage.OriginHeightTag, utils.PackUint64LE(originHeight)); err != nil {
			return err
		}
		if err := bb.Put(storage.OriginHashTag, origin.GetHash()); err != nil {
			return err
		}
		if err := bb.Put(utils.PackUint64LE(originHeight), origin.GetHash()); err != nil {
			return err
		}
		return ngblocks.PutSideBlock(bb, side)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		branch, err := collectBranch(bb, side)
		if err != nil {
			t.Fatalf("collectBranch = %v, want success", err)
		}
		if len(branch) != 1 || branch[0].GetHeight() != originHeight+1 {
			t.Fatalf("branch = %v", branch)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// collectBranch surfaces a broken link: a side block whose prev is not stored
// (and is not the origin) fails with a wrapped not-connected error.
func TestCollectBranchNotConnected(t *testing.T) {
	db := internalDB(t)
	miner := mustKey(t)

	// origin is the genesis at height 0
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	// a side block high above the origin whose prev is absent
	orphan := sealBlock(t, 5, make([]byte, 32), miner)

	if err := db.Update(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if err := bb.Put(storage.OriginHeightTag, utils.PackUint64LE(0)); err != nil {
			return err
		}
		if err := bb.Put(storage.OriginHashTag, genesis.GetHash()); err != nil {
			return err
		}
		return ngblocks.PutSideBlock(bb, orphan)
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if _, err := collectBranch(bb, orphan); err == nil {
			t.Fatal("collectBranch must fail on a disconnected branch")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// collectBranch returns the origin-height lookup error when the origin tag is
// missing.
func TestCollectBranchNoOriginHeight(t *testing.T) {
	db := internalDB(t)
	miner := mustKey(t)
	side := sealBlock(t, 1, make([]byte, 32), miner)

	if err := db.View(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if _, err := collectBranch(bb, side); err == nil {
			t.Fatal("collectBranch must fail without an origin-height tag")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// cumulativeWork returns the not-connected error when a prev link is missing
// before any memoized/origin base is reached.
func TestCumulativeWorkBrokenChain(t *testing.T) {
	db := internalDB(t)
	miner := mustKey(t)

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	// a block whose prev is not stored and is not the origin
	block := sealBlock(t, 5, make([]byte, 32), miner)

	if err := db.Update(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if err := bb.Put(storage.OriginHeightTag, utils.PackUint64LE(0)); err != nil {
			return err
		}
		return bb.Put(storage.OriginHashTag, genesis.GetHash())
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if _, err := cumulativeWork(bb, block); err == nil {
			t.Fatal("cumulativeWork must fail on a broken chain")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// cumulativeWork returns the origin-hash lookup error when no memoized work
// exists and the origin tag is missing.
func TestCumulativeWorkNoOrigin(t *testing.T) {
	db := internalDB(t)
	miner := mustKey(t)
	block := sealBlock(t, 1, make([]byte, 32), miner)

	if err := db.View(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		if _, err := cumulativeWork(bb, block); err == nil {
			t.Fatal("cumulativeWork must fail without an origin tag")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
