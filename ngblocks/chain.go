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

// BlockStore managers a badger DB, which stores vaults and blocks and some helper tags for managing.
// TODO: Add DAG support to extend the capacity of store
type BlockStore struct {
	*badger.DB
}

var store *BlockStore

// Init will return a store, but no initialization.
func Init(db *badger.DB) {
	if store == nil {
		store = &BlockStore{
			db,
		}
	}
}
