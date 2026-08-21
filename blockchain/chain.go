package blockchain

import (
	logging "github.com/ngchain/zap-log"
	"go.etcd.io/bbolt"

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

	// OnReorg (optional) runs post-commit after a reorg with the logs that
	// were in the orphaned blocks, so a logs subscription can notify them
	// as removed. reorgRemoved carries them out of the write txn
	OnReorg      func(removed []ngstate.Log)
	reorgRemoved []ngstate.Log
}

// notifyTipChanged fires the OnTipChanged hook when set
func (chain *Chain) notifyTipChanged() {
	if chain.OnTipChanged != nil {
		chain.OnTipChanged()
	}
}

// notifyReorg fires the OnReorg hook with the logs orphaned by the last
// reorg (gathered inside switchToBranchTxn), then clears them. Reorgs are
// serialized by the write lock, so reorgRemoved is single-owner; the entry
// points reset it before their txn, so an aborted reorg cannot leak stale
// logs into a later fire
func (chain *Chain) notifyReorg() {
	if chain.OnReorg != nil && len(chain.reorgRemoved) > 0 {
		chain.OnReorg(chain.reorgRemoved)
	}
	chain.reorgRemoved = nil
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
