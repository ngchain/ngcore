package storage

import (
	"os"
	"path"

	"github.com/c0mm4nd/dbolt"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("storage")

var db *dbolt.DB

// InitStorage inits a new DB in data folder.
func InitStorage(network ngtypes.Network, dbFolder string) *dbolt.DB {
	if db == nil {
		err := os.MkdirAll(dbFolder, os.ModePerm)
		if err != nil {
			log.Panic(err)
		}
		dbFilePath := path.Join(dbFolder, network.String()+".db")

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
