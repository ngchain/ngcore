package storage

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/whyrusleeping/go-logging"
)

var log = logging.MustGetLogger("storage")

func InitStorage() *badger.DB {
	s, err := badger.Open(badger.DefaultOptions("data"))
	if err != nil {
		log.Panic("failed to open blockchain.db:", err)
	}
	return s
}

func InitMemStorage() *badger.DB {
	s, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
	if err != nil {
		log.Panic("failed to open blockchain.db:", err)
	}
	return s
}
