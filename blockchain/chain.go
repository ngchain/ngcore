package blockchain

import (
	"go.etcd.io/bbolt"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("chain")

type Chain struct {
	*bbolt.DB

	*ngblocks.BlockStore
	*ngstate.State

	Network ngtypes.Network

	// OnTipChanged (optional) runs after the canonical tip moved — on
	// block imports and reorgs, once the db txn has committed. The tx
	// pool hooks in here: its txs are height-locked, so any tip change
	// deprecates them
	OnTipChanged func()
}

// notifyTipChanged fires the OnTipChanged hook when set
func (chain *Chain) notifyTipChanged() {
	if chain.OnTipChanged != nil {
		chain.OnTipChanged()
	}
}

func Init(db *bbolt.DB, network ngtypes.Network, store *ngblocks.BlockStore, state *ngstate.State) *Chain {
	chain := &Chain{
		DB: db,

		BlockStore: store,
		State:      state,

		Network: network,
	}

	return chain
}
