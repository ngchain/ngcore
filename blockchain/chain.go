package blockchain

import (
	"github.com/c0mm4nd/dbolt"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("chain")

type Chain struct {
	*dbolt.DB

	*ngblocks.BlockStore
	*ngstate.State

	Network ngtypes.Network
}

func Init(db *dbolt.DB, network ngtypes.Network, store *ngblocks.BlockStore, state *ngstate.State) *Chain {
	chain := &Chain{
		DB: db,

		BlockStore: store,
		State:      state,

		Network: network,
	}

	return chain
}
