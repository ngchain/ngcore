package storage

import (
	"github.com/dgraph-io/badger/v2"
)

const (
	latestHeightTag = "height"
	latestHashTag   = "hash"
)

var (
	blockPrefix = []byte("b")
	txPrefix    = []byte("t")
)

// Chain managers a badger DB, which stores vaults and blocks and some helper tags for managing.
type Chain struct {
	db *badger.DB
}

var chain *Chain

// NewChain will return a chain, but no initialization.
func NewChain(db *badger.DB) *Chain {
	if chain == nil {
		chain = &Chain{
			db: db,
		}
	}

	return chain
}

func GetChain() *Chain {
	if chain == nil {
		panic("chain is closed")
	}

	return chain
}
