package ngstate

import (
	"sync"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("sheet")

// State is a global set of account & txs status
// TODO: Add TxPool and manage all off-chain data & structure in State
type State struct {
	sync.RWMutex

	prevBlockHash []byte

	// using bytes to keep data safe
	accounts  map[uint64][]byte
	anonymous map[string][]byte // key is base58encoded

	pool *TxPool
}

func (s *State) GetPool() *TxPool {
	return s.pool
}

// GetStaticState will create a Sheet manager
func GetStaticState() *State {
	if manager == nil {
		panic("failed to get current state from nil")
	}

	if manager.GetStaticState() == nil {
		panic("failed to get prev state from nil")
	}

	return manager.staticState
}

// GetActiveState will create a Sheet manager
func GetActiveState() *State {
	if manager == nil {
		panic("failed to get current state from nil")
	}

	if manager.GetCurrentState() == nil {
		panic("failed to get current state from nil")
	}

	return manager.activeState
}
