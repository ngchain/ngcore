package blockchain

import (
	"github.com/dgraph-io/badger/v3"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngtypes/ngproto"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
)

var log = logging.Logger("chain")

type Chain struct {
	*badger.DB

	*ngblocks.BlockStore
	*ngstate.State

	Network ngproto.NetworkType
}

func Init(db *badger.DB, network ngproto.NetworkType, store *ngblocks.BlockStore, state *ngstate.State) *Chain {
	chain := &Chain{
		DB: db,

		BlockStore: store,
		State:      state,

		Network: network,
	}

	return chain
}
