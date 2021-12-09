package storage

import (
	"path"

	"github.com/c0mm4nd/dbolt"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("storage")

var db *dbolt.DB

// InitStorage inits a new DB in data folder.
func InitStorage(network ngtypes.Network, dbFolder string) *dbolt.DB {
	if db == nil {
		dbFilePath := path.Join(dbFolder, network.String()+".db")

		var err error
		db, err = dbolt.Open(dbFilePath, 0666, nil)
		if err != nil {
			log.Panic("failed to init dboltDB:", err)
		}

		InitDB(db)
	}

	return db
}
func InitTempStorage() *dbolt.DB {
	if db == nil {
		var err error

		db, err = dbolt.OpenTemp("ngcore.*.db", nil)
		if err != nil {
			log.Panic("failed to init dboltDB:", err)
		}

		InitDB(db)
	}

	return db
}
