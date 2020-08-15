package storage_test

import (
	"github.com/ngchain/ngcore/storage"
	"testing"
)

func TestInitStorage(t *testing.T) {
	db := storage.InitStorage(".ngdb")
	if db == nil {
		t.Error("failed to init db on home dir")
		return
	}

	memdb := storage.InitMemStorage()
	if memdb == nil {
		t.Error("failed to init db on mem")
		return
	}

}
