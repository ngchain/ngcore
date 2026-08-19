package ngblocks

import (
	"bytes"
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// newDB opens a fresh bbolt db in a temp dir with all buckets created.
func newDB(t *testing.T) *bbolt.DB {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "blocks.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)

	return db
}

// buildBlock crafts a hash-stable (sealed but not necessarily pow-valid)
// block on the parent: ngblocks only checks linkage, not pow
func buildBlock(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16
	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(), diff)

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
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

func newKey(t *testing.T) *ngtypes.PrivateKey {
	t.Helper()

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// update runs fn inside a write txn handing over both buckets.
func update(t *testing.T, db *bbolt.DB, fn func(blockBucket, txBucket *bbolt.Bucket) error) {
	t.Helper()

	if err := db.Update(func(txn *bbolt.Tx) error {
		return fn(txn.Bucket(storage.BlockBucketName), txn.Bucket(storage.TxBucketName))
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInitWritesGenesis(t *testing.T) {
	db := newDB(t)

	store := Init(db, ngtypes.ZERONET)
	if store == nil {
		t.Fatal("store is nil")
	}

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		latest, err := GetLatestBlock(blockBucket)
		if err != nil {
			t.Fatalf("latest block: %v", err)
		}
		if !bytes.Equal(latest.GetHash(), genesis.GetHash()) {
			t.Fatal("latest block is not genesis")
		}

		origin, err := GetOriginBlock(blockBucket)
		if err != nil {
			t.Fatalf("origin block: %v", err)
		}
		if !bytes.Equal(origin.GetHash(), genesis.GetHash()) {
			t.Fatal("origin block is not genesis")
		}

		if h, err := GetLatestHeight(blockBucket); err != nil || h != 0 {
			t.Fatalf("latest height = %d, %v; want 0, nil", h, err)
		}
		if h, err := GetOriginHeight(blockBucket); err != nil || h != 0 {
			t.Fatalf("origin height = %d, %v; want 0, nil", h, err)
		}

		hash, err := GetLatestHash(blockBucket)
		if err != nil || !bytes.Equal(hash, genesis.GetHash()) {
			t.Fatalf("latest hash = %x, %v", hash, err)
		}
		hash, err = GetOriginHash(blockBucket)
		if err != nil || !bytes.Equal(hash, genesis.GetHash()) {
			t.Fatalf("origin hash = %x, %v", hash, err)
		}

		byHeight, err := GetBlockByHeight(blockBucket, 0)
		if err != nil || !bytes.Equal(byHeight.GetHash(), genesis.GetHash()) {
			t.Fatalf("block@0 = %v, %v", byHeight, err)
		}

		return nil
	})
}

func TestInitSecondOpenKeepsGenesis(t *testing.T) {
	db := newDB(t)

	miner := newKey(t)
	store := Init(db, ngtypes.ZERONET)
	b1 := buildBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return PutNewBlock(blockBucket, txBucket, b1)
	})

	// reopening the store must NOT reset the chain back to genesis
	store = Init(db, ngtypes.ZERONET)
	if store == nil {
		t.Fatal("store is nil")
	}

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		h, err := GetLatestHeight(blockBucket)
		if err != nil {
			return err
		}
		if h != 1 {
			t.Fatalf("latest height = %d after reopen, want 1", h)
		}
		return nil
	})
}

func TestInitPanicsOnTamperedGenesis(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	// tamper the canonical hash at height 0
	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		return blockBucket.Put(utils.PackUint64LE(0), make([]byte, 32))
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Init on a tampered genesis must panic")
		}
		err, ok := r.(error)
		if !ok || !errors.Is(err, ErrMalformedGenesisBlock) {
			t.Fatalf("panic value = %v, want ErrMalformedGenesisBlock", r)
		}
	}()

	Init(db, ngtypes.ZERONET)
}

func TestHasGenesisBlockOnUninitializedDB(t *testing.T) {
	// raw db without any bucket: hasGenesisBlock must report false
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "raw.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := &BlockStore{DB: db, Network: ngtypes.ZERONET}
	if store.hasGenesisBlock(ngtypes.ZERONET) {
		t.Fatal("uninitialized db cannot contain the genesis block")
	}
}

func TestPutNewBlockAndGetters(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	b2 := buildBlock(t, b1, miner)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err != nil {
			t.Fatalf("put b1: %v", err)
		}
		if err := PutNewBlock(blockBucket, txBucket, b2); err != nil {
			t.Fatalf("put b2: %v", err)
		}

		// block getters
		got, err := GetBlockByHeight(blockBucket, 1)
		if err != nil || !bytes.Equal(got.GetHash(), b1.GetHash()) {
			t.Fatalf("block@1: %v, %v", got, err)
		}
		got, err = GetBlockByHash(blockBucket, b2.GetHash())
		if err != nil || got.GetHeight() != 2 {
			t.Fatalf("block by hash: %v, %v", got, err)
		}
		latest, err := GetLatestBlock(blockBucket)
		if err != nil || !bytes.Equal(latest.GetHash(), b2.GetHash()) {
			t.Fatal("latest block should be b2")
		}
		if h, _ := GetLatestHeight(blockBucket); h != 2 {
			t.Fatalf("latest height = %d, want 2", h)
		}

		// tx getters
		txHash := b1.Txs[0].GetHash()
		tx, err := GetTxByHash(txBucket, txHash)
		if err != nil {
			t.Fatalf("get tx: %v", err)
		}
		if !bytes.Equal(tx.GetHash(), txHash) {
			t.Fatal("tx hash mismatch after decode")
		}
		blockHash, err := GetTxBlockHash(txBucket, txHash)
		if err != nil || !bytes.Equal(blockHash, b1.GetHash()) {
			t.Fatalf("tx block hash = %x, %v; want b1", blockHash, err)
		}

		return nil
	})
}

func TestPutNewBlockRejections(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	b2 := buildBlock(t, b1, miner)
	b3 := buildBlock(t, b2, miner)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, nil); !errors.Is(err, ErrPutEmptyBlock) {
			t.Fatalf("nil block: got %v, want ErrPutEmptyBlock", err)
		}

		// b2 before b1: prev block missing
		if err := PutNewBlock(blockBucket, txBucket, b2); !errors.Is(err, ErrPrevBlockNotExist) {
			t.Fatalf("orphan: got %v, want ErrPrevBlockNotExist", err)
		}

		if err := PutNewBlock(blockBucket, txBucket, b1); err != nil {
			t.Fatal(err)
		}

		// same height again: conflict
		alt1 := buildBlock(t, genesis, newKey(t))
		if err := PutNewBlock(blockBucket, txBucket, alt1); !errors.Is(err, ErrBlockHeightConflict) {
			t.Fatalf("height conflict: got %v, want ErrBlockHeightConflict", err)
		}

		// b3 skipping b2: prev block missing
		if err := PutNewBlock(blockBucket, txBucket, b3); !errors.Is(err, ErrPrevBlockNotExist) {
			t.Fatalf("gap: got %v, want ErrPrevBlockNotExist", err)
		}

		return nil
	})
}

func TestCheckBlockBranches(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err != nil {
			t.Fatal(err)
		}

		// height 0 always "exists" (genesis)
		if err := checkBlock(blockBucket, 0, make([]byte, 32)); !errors.Is(err, ErrBlockHeightConflict) {
			t.Fatalf("genesis height: got %v, want ErrBlockHeightConflict", err)
		}

		// genesis prev rule: height 0 + zero hash is a valid prev
		if !blockPrevHashExists(blockBucket, 0, make([]byte, 32)) {
			t.Fatal("genesis zero prev must be accepted")
		}

		// prev exists but at the wrong height
		if err := checkBlock(blockBucket, 5, b1.GetHash()); !errors.Is(err, ErrPrevBlockNotExist) {
			t.Fatalf("wrong-height prev: got %v, want ErrPrevBlockNotExist", err)
		}

		// a corrupted block at some height still counts as existing
		if err := blockBucket.Put(utils.PackUint64LE(9), b1.GetHash()); err != nil {
			t.Fatal(err)
		}
		if err := blockBucket.Put(b1.GetHash(), []byte{0xff}); err != nil {
			t.Fatal(err)
		}
		if !blockHeightExists(blockBucket, 9) {
			t.Fatal("undecodable block must still mark the height as occupied")
		}

		return nil
	})
}

func TestGettersNotFoundAndDecodeErrors(t *testing.T) {
	db := newDB(t) // buckets exist but the store is never initialized

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		unknown := bytes.Repeat([]byte{0xaa}, 32)

		if _, err := GetBlockByHash(blockBucket, unknown); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("block by hash: %v", err)
		}
		if _, err := GetBlockByHeight(blockBucket, 42); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("block by height: %v", err)
		}
		if _, err := GetLatestHeight(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("latest height: %v", err)
		}
		if _, err := GetLatestHash(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("latest hash: %v", err)
		}
		if _, err := GetLatestBlock(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("latest block: %v", err)
		}
		if _, err := GetOriginHeight(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("origin height: %v", err)
		}
		if _, err := GetOriginHash(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("origin hash: %v", err)
		}
		if _, err := GetOriginBlock(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("origin block: %v", err)
		}
		if _, err := GetTxByHash(txBucket, unknown); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("tx by hash: %v", err)
		}
		if _, err := GetTxBlockHash(txBucket, unknown); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("tx block hash: %v", err)
		}

		// undecodable payloads
		if err := blockBucket.Put(unknown, []byte{0xff}); err != nil {
			t.Fatal(err)
		}
		if _, err := GetBlockByHash(blockBucket, unknown); err == nil {
			t.Fatal("garbage block must fail to decode")
		}
		if err := blockBucket.Put(utils.PackUint64LE(7), unknown); err != nil {
			t.Fatal(err)
		}
		if _, err := GetBlockByHeight(blockBucket, 7); err == nil {
			t.Fatal("garbage block behind a height must fail to decode")
		}
		if err := txBucket.Put(unknown, []byte{0xff}); err != nil {
			t.Fatal(err)
		}
		if _, err := GetTxByHash(txBucket, unknown); err == nil {
			t.Fatal("garbage tx must fail to decode")
		}

		// GetLatestBlock with a dangling latest hash
		if err := blockBucket.Put(storage.LatestHashTag, bytes.Repeat([]byte{0xbb}, 32)); err != nil {
			t.Fatal(err)
		}
		if _, err := GetLatestBlock(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("dangling latest hash: %v", err)
		}
		// GetOriginBlock with a dangling origin hash
		if err := blockBucket.Put(storage.OriginHashTag, bytes.Repeat([]byte{0xcc}, 32)); err != nil {
			t.Fatal(err)
		}
		if _, err := GetOriginBlock(blockBucket); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("dangling origin hash: %v", err)
		}

		return nil
	})
}

func TestForcePutNewBlock(t *testing.T) {
	db := newDB(t)
	store := Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	alt1 := buildBlock(t, genesis, newKey(t))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := store.ForcePutNewBlock(blockBucket, txBucket, b1); err != nil {
			t.Fatalf("force put b1: %v", err)
		}

		// overwrite height 1: old txs must leave the index
		oldTxHash := b1.Txs[0].GetHash()
		if err := store.ForcePutNewBlock(blockBucket, txBucket, alt1); err != nil {
			t.Fatalf("force put alt1: %v", err)
		}
		if _, err := GetTxByHash(txBucket, oldTxHash); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("replaced tx must be deleted, got %v", err)
		}

		got, err := GetBlockByHeight(blockBucket, 1)
		if err != nil || !bytes.Equal(got.GetHash(), alt1.GetHash()) {
			t.Fatal("height 1 should map to alt1")
		}
		hash, err := GetLatestHash(blockBucket)
		if err != nil || !bytes.Equal(hash, alt1.GetHash()) {
			t.Fatal("latest tag should point at alt1")
		}

		// unknown prev is rejected
		b2 := buildBlock(t, b1, miner)
		b3 := buildBlock(t, b2, miner)
		if err := store.ForcePutNewBlock(blockBucket, txBucket, b3); !errors.Is(err, ErrPrevBlockNotExist) {
			t.Fatalf("unknown prev: got %v, want ErrPrevBlockNotExist", err)
		}

		return nil
	})
}

func TestForcePutNilBlockPanics(t *testing.T) {
	db := newDB(t)
	store := Init(db, ngtypes.ZERONET)

	defer func() {
		if recover() == nil {
			t.Fatal("nil block must panic")
		}
	}()

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		return store.ForcePutNewBlock(blockBucket, txBucket, nil)
	})
}

func TestInitFromCheckpoint(t *testing.T) {
	db := newDB(t)
	store := Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	b2 := buildBlock(t, b1, miner)

	if err := store.InitFromCheckpoint(b2); err != nil {
		t.Fatalf("init from checkpoint: %v", err)
	}

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		origin, err := GetOriginBlock(blockBucket)
		if err != nil || !bytes.Equal(origin.GetHash(), b2.GetHash()) {
			t.Fatal("origin should be the checkpoint block")
		}
		if h, _ := GetOriginHeight(blockBucket); h != 2 {
			t.Fatalf("origin height = %d, want 2", h)
		}
		latest, err := GetLatestBlock(blockBucket)
		if err != nil || !bytes.Equal(latest.GetHash(), b2.GetHash()) {
			t.Fatal("latest should be the checkpoint block")
		}
		return nil
	})
}

func TestSwitchToBranch(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	minerA := newKey(t)
	minerB := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := buildBlock(t, genesis, minerA)
	a2 := buildBlock(t, b1, minerA)
	c2 := buildBlock(t, b1, minerB)
	c3 := buildBlock(t, c2, minerB)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := PutNewBlock(blockBucket, txBucket, b1); err != nil {
			t.Fatal(err)
		}
		if err := PutNewBlock(blockBucket, txBucket, a2); err != nil {
			t.Fatal(err)
		}

		if err := SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{c2, c3}); err != nil {
			t.Fatalf("switch: %v", err)
		}

		// the canonical index follows the branch
		got, err := GetBlockByHeight(blockBucket, 2)
		if err != nil || !bytes.Equal(got.GetHash(), c2.GetHash()) {
			t.Fatal("height 2 should map to c2")
		}
		if h, _ := GetLatestHeight(blockBucket); h != 3 {
			t.Fatalf("latest height = %d, want 3", h)
		}
		hash, _ := GetLatestHash(blockBucket)
		if !bytes.Equal(hash, c3.GetHash()) {
			t.Fatal("latest hash should be c3")
		}

		// the replaced block stays by hash, marked as a side block
		if _, err := GetBlockByHash(blockBucket, a2.GetHash()); err != nil {
			t.Fatal("a2 must stay stored by hash")
		}
		if blockBucket.Get(sideBlockKey(a2.GetHash())) == nil {
			t.Fatal("a2 must be marked as a side block")
		}
		// ...but its txs leave the index
		if _, err := GetTxByHash(txBucket, a2.Txs[0].GetHash()); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("a2's txs must be unindexed, got %v", err)
		}
		// promoted branch blocks are not side blocks
		if blockBucket.Get(sideBlockKey(c2.GetHash())) != nil {
			t.Fatal("c2 must not be a side block anymore")
		}
		// branch txs are indexed
		if _, err := GetTxByHash(txBucket, c3.Txs[0].GetHash()); err != nil {
			t.Fatalf("c3's txs must be indexed: %v", err)
		}

		return nil
	})
}

func TestSwitchToBranchTruncatesStaleHeights(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	minerA := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := buildBlock(t, genesis, minerA)
	b2 := buildBlock(t, b1, minerA)
	b3 := buildBlock(t, b2, minerA)
	c2 := buildBlock(t, b1, newKey(t))

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		for _, b := range []*ngtypes.FullBlock{b1, b2, b3} {
			if err := PutNewBlock(blockBucket, txBucket, b); err != nil {
				t.Fatal(err)
			}
		}

		// switch to a SHORTER branch: stale heights must be unlinked
		if err := SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{c2}); err != nil {
			t.Fatalf("switch: %v", err)
		}

		if h, _ := GetLatestHeight(blockBucket); h != 2 {
			t.Fatalf("latest height = %d, want 2", h)
		}
		if _, err := GetBlockByHeight(blockBucket, 3); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("stale height 3 must be unlinked, got %v", err)
		}

		return nil
	})
}

func TestSwitchToBranchRejections(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := buildBlock(t, genesis, miner)
	b2 := buildBlock(t, b1, miner)
	b3 := buildBlock(t, b2, miner)

	update(t, db, func(blockBucket, txBucket *bbolt.Bucket) error {
		if err := SwitchToBranch(blockBucket, txBucket, nil); !errors.Is(err, ErrBranchEmpty) {
			t.Fatalf("empty: got %v, want ErrBranchEmpty", err)
		}

		// disconnected: b3 does not follow b1
		if err := SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{b1, b3}); !errors.Is(err, ErrBranchDisconnected) {
			t.Fatalf("disconnected: got %v, want ErrBranchDisconnected", err)
		}

		// detached: b2's parent b1 is not canonical
		if err := SwitchToBranch(blockBucket, txBucket, []*ngtypes.FullBlock{b2, b3}); !errors.Is(err, ErrBranchDetached) {
			t.Fatalf("detached: got %v, want ErrBranchDetached", err)
		}

		return nil
	})
}

func TestBlockWork(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	hash := bytes.Repeat([]byte{0x11}, 32)
	work := big.NewInt(12345)

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		if _, err := GetBlockWork(blockBucket, hash); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatalf("missing work: got %v, want ErrKeyNotFound", err)
		}

		if err := PutBlockWork(blockBucket, hash, work); err != nil {
			t.Fatal(err)
		}

		got, err := GetBlockWork(blockBucket, hash)
		if err != nil || got.Cmp(work) != 0 {
			t.Fatalf("work = %v, %v; want %v", got, err, work)
		}

		return nil
	})
}

func TestSideBlocksAndPruning(t *testing.T) {
	db := newDB(t)
	Init(db, ngtypes.ZERONET)

	miner := newKey(t)
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	side1 := buildBlock(t, genesis, miner)
	side2 := buildBlock(t, side1, miner)

	update(t, db, func(blockBucket, _ *bbolt.Bucket) error {
		if err := PutSideBlock(blockBucket, nil); !errors.Is(err, ErrPutEmptyBlock) {
			t.Fatalf("nil side block: got %v, want ErrPutEmptyBlock", err)
		}

		if err := PutSideBlock(blockBucket, side1); err != nil {
			t.Fatal(err)
		}
		if err := PutSideBlock(blockBucket, side2); err != nil {
			t.Fatal(err)
		}
		if err := PutBlockWork(blockBucket, side1.GetHash(), big.NewInt(1)); err != nil {
			t.Fatal(err)
		}

		// side blocks are reachable by hash but NOT canonical
		if _, err := GetBlockByHash(blockBucket, side1.GetHash()); err != nil {
			t.Fatal("side block must be stored by hash")
		}
		if hash := blockBucket.Get(utils.PackUint64LE(1)); hash != nil {
			t.Fatal("side block must not enter the canonical height index")
		}

		// prune below height 2: side1 goes, side2 stays
		pruned, err := PruneSideBlocks(blockBucket, 2)
		if err != nil {
			t.Fatal(err)
		}
		if pruned != 1 {
			t.Fatalf("pruned = %d, want 1", pruned)
		}
		if _, err := GetBlockByHash(blockBucket, side1.GetHash()); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatal("side1 must be pruned")
		}
		if _, err := GetBlockWork(blockBucket, side1.GetHash()); !errors.Is(err, storage.ErrKeyNotFound) {
			t.Fatal("side1's work entry must be pruned")
		}
		if _, err := GetBlockByHash(blockBucket, side2.GetHash()); err != nil {
			t.Fatal("side2 must survive")
		}

		// nothing left below the line
		pruned, err = PruneSideBlocks(blockBucket, 2)
		if err != nil || pruned != 0 {
			t.Fatalf("second prune = %d, %v; want 0, nil", pruned, err)
		}

		return nil
	})
}
