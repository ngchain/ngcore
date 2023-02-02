package storage

import (
	"os"
	"path"
	"strconv"
	"time"

	logging "github.com/ngchain/zap-log"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("storage")

var db *bbolt.DB

// InitStorage inits a new DB in data folder.
func InitStorage(network ngtypes.Network, dbFolder string) *bbolt.DB {
	if db == nil {
		err := os.MkdirAll(dbFolder, os.ModePerm)
		if err != nil {
			log.Panic(err)
		}
		dbFilePath := path.Join(dbFolder, network.String()+".db")

		db, err = bbolt.Open(dbFilePath, 0666, nil)
		if err != nil {
			log.Panic("failed to init dboltDB:", err)
		}

		InitDB(db)
	}

	return db
}

func InitTempStorage() *bbolt.DB {
	if db == nil {
		var err error

		path := path.Join(os.TempDir(), "ngcore_"+strconv.FormatInt(time.Now().UTC().Unix(), 10))
		db, err = bbolt.Open(path, 0666, nil)
		if err != nil {
			log.Panic("failed to init dboltDB:", err)
		}

		InitDB(db)
	}

	return db
}
