package ngchain

import (
	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("chain")

type Chain struct {
	*badger.DB
}

var chain *Chain

func Init(db *badger.DB) {
	chain = &Chain{db}
}
