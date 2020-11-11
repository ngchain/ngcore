package ngblocks

import (
	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngtypes"
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
	Network ngtypes.NetworkType
}

// Init will do all initialization for the block store.
func Init(db *badger.DB, network ngtypes.NetworkType, blocks ...*ngtypes.Block) *BlockStore {
	store := &BlockStore{
		DB:      db,
		Network: network,
	}

	if blocks == nil {
		store.initWithGenesis()
	} else {
		err := store.initWithBlockchain(blocks...)
		if err != nil {
			panic(err)
		}
	}

	return store
}
