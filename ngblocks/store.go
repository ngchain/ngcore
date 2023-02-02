package ngblocks

import (
	"go.etcd.io/bbolt"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("blocks")

// BlockStore managers a badger DB, which stores vaults and blocks and some helper tags for managing.
// TODO: Add DAG support to extend the capacity of store
// initialize with genesis blocks first,
// then load the origin in bootstrap process
type BlockStore struct {
	*bbolt.DB
	Network ngtypes.Network
}

// Init will do all initialization for the block store.
func Init(db *bbolt.DB, network ngtypes.Network) *BlockStore {
	store := &BlockStore{
		DB:      db,
		Network: network,
	}

	store.initWithGenesis()
	// err := store.initWithBlockchain(blocks...)
	// if err != nil {
	//		panic(err)
	//	}

	return store
}
