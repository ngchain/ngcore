package storage

import (
	"runtime"

	"github.com/dgraph-io/badger/v2"
	"github.com/whyrusleeping/go-logging"
)

var log = logging.MustGetLogger("storage")

// InitStorage inits a new DB in data folder
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

// InitMemStorage inits a new DB in mem
// TODO: add memdb mode
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
