package storage

import (
	"path"

	"github.com/dgraph-io/badger/v3"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("storage")

var db *badger.DB

// InitStorage inits a new DB in data folder.
func InitStorage(network ngtypes.Network, dbFolder string) *badger.DB {
	if db == nil {
		options := badger.DefaultOptions(path.Join(dbFolder, network.String()))

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
		options.Logger = log

		var err error

		db, err = badger.Open(options)
		if err != nil {
			log.Panic("failed to init badgerDB:", err)
		}
	}

	return db
}
