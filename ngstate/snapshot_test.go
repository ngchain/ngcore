package ngstate

import (
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func testSheet(height uint64, hash []byte) *ngtypes.Sheet {
	return ngtypes.NewSheet(ngtypes.ZERONET, height, hash,
		[]*ngtypes.Balance{{Address: testAddr(byte(height)), Amount: big.NewInt(int64(height))}},
		[]*ngtypes.Contract{}, []*ngtypes.RegisteredKey{})
}

// TestSnapshotManager covers the in-memory snapshot cache: hash-checked
// and unchecked lookups plus the retention pruning
func TestSnapshotManager(t *testing.T) {
	sm := &SnapshotManager{
		heightToHash:   make(map[uint64]string),
		hashToSnapshot: make(map[string]*ngtypes.Sheet),
	}

	hash1 := []byte("hash-one")
	sheet1 := testSheet(1, hash1)
	sm.PutSnapshot(1, hash1, sheet1)

	if got := sm.GetSnapshot(1, hash1); got != sheet1 {
		t.Fatal("GetSnapshot with the right hash must hit")
	}
	if got := sm.GetSnapshot(1, []byte("wrong")); got != nil {
		t.Fatal("GetSnapshot with a wrong hash must miss")
	}
	if got := sm.GetSnapshot(2, hash1); got != nil {
		t.Fatal("GetSnapshot at an unknown height must miss")
	}
	if got := sm.GetSnapshotByHeight(1); got != sheet1 {
		t.Fatal("GetSnapshotByHeight must hit")
	}
	if got := sm.GetSnapshotByHeight(9); got != nil {
		t.Fatal("GetSnapshotByHeight at an unknown height must miss")
	}
	if got := sm.GetSnapshotByHash(hash1); got != sheet1 {
		t.Fatal("GetSnapshotByHash must hit")
	}
	if got := sm.GetSnapshotByHash([]byte("nope")); got != nil {
		t.Fatal("GetSnapshotByHash on an unknown hash must miss")
	}

	// a far-newer snapshot prunes everything below the retention floor
	farHeight := uint64(snapshotRetention*ngtypes.BlockCheckRound + 100)
	farHash := []byte("hash-far")
	sm.PutSnapshot(farHeight, farHash, testSheet(farHeight, farHash))

	if got := sm.GetSnapshotByHeight(1); got != nil {
		t.Fatal("the old snapshot must be pruned")
	}
	if got := sm.GetSnapshotByHeight(farHeight); got == nil {
		t.Fatal("the new snapshot must stay")
	}
}

// TestStatePersistedSnapshots covers the bucket-backed snapshot store:
// exact hits, the nearest-older floor, the genesis fallbacks, broken
// records, and the persisted retention pruning
func TestStatePersistedSnapshots(t *testing.T) {
	db := newTestDB(t)
	ngblocks.Init(db, ngtypes.ZERONET)
	state := newTestState(t, db)

	sheet5 := testSheet(5, []byte("hash-five"))
	err := state.Update(func(txn *bbolt.Tx) error {
		return state.PutSnapshotTxn(txn, sheet5)
	})
	if err != nil {
		t.Fatal(err)
	}

	// the in-mem cache serves it back, hash-checked or not
	if got := state.GetSnapshot(5, []byte("hash-five")); got == nil {
		t.Fatal("state.GetSnapshot must hit the fresh snapshot")
	}

	// a FRESH manager (simulating a restart) must fall through to the
	// persisted bucket
	restarted := newTestState(t, db)

	got := restarted.GetSnapshotByHeight(5)
	if got == nil || got.Height != 5 {
		t.Fatalf("persisted snapshot not resolved: %+v", got)
	}
	// and now it is cached in memory
	if restarted.SnapshotManager.GetSnapshotByHeight(5) == nil {
		t.Fatal("the resolved snapshot must be re-cached")
	}

	// a height above: the nearest OLDER one is the conservative floor
	fresh2 := newTestState(t, db)
	got = fresh2.GetSnapshotByHeight(7)
	if got == nil || got.Height != 5 {
		t.Fatalf("nearest-older floor = %+v, want height 5", got)
	}

	// a height below every persisted snapshot: the genesis sheet
	fresh3 := newTestState(t, db)
	got = fresh3.GetSnapshotByHeight(3)
	if got == nil || got.Height != 0 {
		t.Fatalf("pre-snapshot height must fall back to genesis, got %+v", got)
	}

	// height 0 is always genesis
	if got := state.GetSnapshotByHeight(0); got == nil || got.Height != 0 {
		t.Fatalf("height 0 must be the genesis sheet, got %+v", got)
	}

	// a broken persisted record degrades to the genesis fallback
	err = db.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.SnapshotBucketName).Put(utils.PackUint64LE(9), []byte{0xff})
	})
	if err != nil {
		t.Fatal(err)
	}
	fresh4 := newTestState(t, db)
	got = fresh4.GetSnapshotByHeight(9)
	if got == nil || got.Height != 0 {
		t.Fatalf("broken persisted snapshot must fall back to genesis, got %+v", got)
	}

	// a far-newer persisted snapshot prunes the bucket below the window
	farHeight := uint64(snapshotRetention*ngtypes.BlockCheckRound + 50)
	err = state.Update(func(txn *bbolt.Tx) error {
		return state.PutSnapshotTxn(txn, testSheet(farHeight, []byte("hash-far")))
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.View(func(txn *bbolt.Tx) error {
		if raw := txn.Bucket(storage.SnapshotBucketName).Get(utils.PackUint64LE(5)); raw != nil {
			t.Fatal("the persisted snapshot below the retention floor survived")
		}
		if raw := txn.Bucket(storage.SnapshotBucketName).Get(utils.PackUint64LE(farHeight)); raw == nil {
			t.Fatal("the fresh persisted snapshot is gone")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGenerateSnapshot: a generated snapshot carries the live balances
// and contracts of the tip
func TestGenerateSnapshot(t *testing.T) {
	db := newTestDB(t)
	ngblocks.Init(db, ngtypes.ZERONET)
	state := newTestState(t, db)

	addr := testAddr(0x41)
	err := state.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(88)); err != nil {
			return err
		}
		if err := setContract(txn, nil, ngtypes.NewContract(addr, mustWat(logWat), nil)); err != nil {
			return err
		}
		return state.GenerateSnapshotTxn(txn)
	})
	if err != nil {
		t.Fatal(err)
	}

	sheet := state.SnapshotManager.GetSnapshotByHeight(0)
	if sheet == nil {
		t.Fatal("no snapshot at the tip height")
	}
	foundBal, foundContract := false, false
	for _, b := range sheet.Balances {
		if b.Address == addr && b.Amount.Int64() == 88 {
			foundBal = true
		}
	}
	for _, c := range sheet.Contracts {
		if c.Owner == addr {
			foundContract = true
		}
	}
	if !foundBal || !foundContract {
		t.Fatalf("sheet misses state: balance=%v contract=%v", foundBal, foundContract)
	}
}
