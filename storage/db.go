package storage

import (
	"runtime"

	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("storage")

var db *badger.DB

// InitStorage inits a new DB in data folder.
func InitStorage(dbFolder string) *badger.DB {
	if db == nil {
		options := badger.DefaultOptions(dbFolder)
		if runtime.GOOS == "windows" {
			options.Truncate = true
		}

		options.Logger = log

		var err error
		db, err = badger.Open(options)
		if err != nil {
			log.Panic("failed to init badgerDB:", err)
		}
	}

	return db
}

// InitMemStorage inits a new DB in mem.
func InitMemStorage() *badger.DB {
	if db == nil {
		options := badger.DefaultOptions("").WithInMemory(true)
		if runtime.GOOS == "windows" {
			options.Truncate = true
		}
		options.Logger = log

		var err error

		db, err = badger.Open(options)
		if err != nil {
			log.Panic("failed to init badgerDB:", err)
		}
	}

	return db
}
