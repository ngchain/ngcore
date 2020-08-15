package ngblocks

import (
	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
)

const (
	latestHeightTag = "height"
	latestHashTag   = "hash"
)

var log = logging.Logger("blocks")

var (
	blockPrefix = []byte("b:")
	txPrefix    = []byte("t:")
)

// Chain managers a badger DB, which stores vaults and blocks and some helper tags for managing.
// TODO: Add DAG support to extend the capacity of chain
type Chain struct {
	*badger.DB
}

var chain *Chain

// NewChain will return a chain, but no initialization.
func NewChain(db *badger.DB) *Chain {
	if chain == nil {
		chain = &Chain{
			db,
		}
	}

	return chain
}

// GetChain returns an unique chain storage
func GetChain() *Chain {
	if chain == nil {
		panic("chain is closed")
	}

	return chain
}
