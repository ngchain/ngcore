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

	prevSheetkHash []byte
	height         uint64

	// using bytes to keep data safe
	accounts  map[uint64][]byte
	anonymous map[string][]byte

	pool *TxPool
}

// GetPrevState will create a Sheet manager
func GetPrevState() *State {
	if manager == nil {
		panic("failed to get current state from nil")
	}

	if manager.prevState == nil {
		panic("failed to get prev state from nil")
	}

	return manager.prevState
}

// GetCurrentState will create a Sheet manager
func GetCurrentState() *State {
	if manager == nil {
		panic("failed to get current state from nil")
	}

	if manager.CurrentState == nil {
		panic("failed to get current state from nil")
	}

	return manager.CurrentState
}
