package storage

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/whyrusleeping/go-logging"
	"runtime"
)

var log = logging.MustGetLogger("storage")

func InitStorage() *badger.DB {
	options := badger.DefaultOptions("data")
	if runtime.GOOS == "windows" {
		options.Truncate = true
	}
	options.Logger = log

	s, err := badger.Open(options)
	if err != nil {
		log.Panic("failed to open storage:", err)
	}
	return s
}

func InitMemStorage() *badger.DB {
	options := badger.DefaultOptions("").WithInMemory(true)
	if runtime.GOOS == "windows" {
		options.Truncate = true
	}
	options.Logger = log

	s, err := badger.Open(options)
	if err != nil {
		log.Panic("failed to open storage:", err)
	}
	return s
}
