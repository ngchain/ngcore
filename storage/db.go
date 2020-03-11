package storage

import (
	"github.com/whyrusleeping/go-logging"
	"go.etcd.io/bbolt"
)

var log = logging.MustGetLogger("storage")

func InitStorage() *bbolt.DB {
	s, err := bbolt.Open("ngcore.db", 0600, nil)
	if err != nil {
		log.Panic("failed to open blockchain.db:", err)
	}
	return s
}
