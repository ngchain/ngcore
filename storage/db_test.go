package storage_test

import (
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

func TestInitStorage(t *testing.T) {
	db := storage.InitStorage(ngtypes.ZERONET, ".")
	if db == nil {
		t.Error("failed to init db on home dir")
		return
	}

	memdb := storage.InitTempStorage()
	if memdb == nil {
		t.Error("failed to init db on mem")
		return
	}
}
