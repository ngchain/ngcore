package storage

import (
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

// TestInitDBPanicsOnFailure: bucket-creation failure must surface as a
// loud panic at the source, not a silently-swallowed error that turns
// into a nil-bucket panic far away. A read-only db makes CreateBucket fail.
func TestInitDBPanicsOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.db")
	db, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	roDB, err := bbolt.Open(path, 0o600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = roDB.Close()
		if r := recover(); r == nil {
			t.Fatal("InitDB on a read-only db should panic, not swallow the error")
		}
	}()
	InitDB(roDB)
}
