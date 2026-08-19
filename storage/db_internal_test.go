package storage

import (
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// resetDB isolates a test from the package-level db singleton: the test
// starts with a nil handle and the previous handle is restored afterwards.
func resetDB(t *testing.T) {
	t.Helper()

	old := db
	db = nil
	t.Cleanup(func() {
		if db != nil && db != old {
			_ = db.Close()
		}
		db = old
	})
}

func TestInitStorageFresh(t *testing.T) {
	resetDB(t)

	dir := t.TempDir()
	got := InitStorage(ngtypes.ZERONET, dir)
	if got == nil {
		t.Fatal("InitStorage returned nil")
	}

	// the db file lands in the requested folder, named after the network
	if _, err := os.Stat(filepath.Join(dir, ngtypes.ZERONET.String()+".db")); err != nil {
		t.Fatalf("db file missing: %v", err)
	}

	// all buckets are created
	if err := got.View(func(txn *bbolt.Tx) error {
		for _, name := range [][]byte{
			BlockBucketName, TxBucketName,
			ContractBucketName, CodeBucketName, Addr2BalBucketName,
			KeyRegistryBucketName,
			SnapshotBucketName, ReceiptBucketName,
		} {
			if txn.Bucket(name) == nil {
				t.Fatalf("bucket %s missing", name)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// the handle is a singleton: a second call returns the same db
	if again := InitStorage(ngtypes.ZERONET, t.TempDir()); again != got {
		t.Fatal("second InitStorage must return the same handle")
	}
}

func TestInitTempStorageFresh(t *testing.T) {
	resetDB(t)

	got := InitTempStorage()
	if got == nil {
		t.Fatal("InitTempStorage returned nil")
	}
	t.Cleanup(func() { _ = os.Remove(got.Path()) })

	if err := got.View(func(txn *bbolt.Tx) error {
		if txn.Bucket(BlockBucketName) == nil {
			t.Fatal("blocks bucket missing")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if again := InitTempStorage(); again != got {
		t.Fatal("second InitTempStorage must return the same handle")
	}
}

func TestInitStoragePanicsOnBadFolder(t *testing.T) {
	resetDB(t)

	// the folder path runs through a regular file: MkdirAll must fail
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("InitStorage must panic when the folder cannot be created")
		}
	}()

	InitStorage(ngtypes.ZERONET, filepath.Join(blocker, "sub"))
}

func TestInitStoragePanicsOnUnopenableFile(t *testing.T) {
	resetDB(t)

	// the db path is occupied by a directory: bbolt.Open must fail
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ngtypes.ZERONET.String()+".db"), 0o755); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("InitStorage must panic when the db file cannot be opened")
		}
	}()

	InitStorage(ngtypes.ZERONET, dir)
}

func TestInitTempStoragePanicsOnBadTempDir(t *testing.T) {
	resetDB(t)

	// point os.TempDir at a non-existent location
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does", "not", "exist"))

	defer func() {
		if recover() == nil {
			t.Fatal("InitTempStorage must panic when the temp db cannot be opened")
		}
	}()

	InitTempStorage()
}
