package ngchain

import (
	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngchain")

const (
	latestHeightTag = "height"
	latestHashTag   = "hash"
)

var (
	blockPrefix = []byte("b")
	txPrefix    = []byte("t")
)

// Chain managers a badger DB, which stores vaults and blocks and some helper tags for managing
type Chain struct {
	db *badger.DB

	MinedBlockToP2PCh    chan *ngtypes.Block
	MinedBlockToTxPoolCh chan *ngtypes.Block
}

// NewChain will return a chain, but no initialization
func NewChain(db *badger.DB) *Chain {
	chain := &Chain{
		db: db,

		MinedBlockToP2PCh:    make(chan *ngtypes.Block),
		MinedBlockToTxPoolCh: make(chan *ngtypes.Block),
	}

	return chain
}
