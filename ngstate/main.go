package ngstate

import (
	"sync"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("sheet")

// State is a global set of account & txs status
// TODO: Add TxPool and manage all off-chain data & structure in State
type State struct {
	sync.RWMutex

	height uint64
	// using bytes to keep data safe
	accounts  map[uint64][]byte
	anonymous map[string][]byte
}

var currentState *State

// GetCurrentState will create a Sheet manager
func GetCurrentState() *State {
	if currentState == nil {
		panic("failed to get current state from nil")
	}

	return currentState
}

// NewStateFromSheet will create a new state which is a wrapper of *ngtypes.sheet
func NewStateFromSheet(sheet *ngtypes.Sheet) (*State, error) {
	entry := &State{
		height:    sheet.Height,
		accounts:  make(map[uint64][]byte),
		anonymous: make(map[string][]byte),
	}

	var err error
	for id, account := range sheet.Accounts {
		entry.accounts[id], err = utils.Proto.Marshal(account)
		if err != nil {
			return nil, err
		}
	}

	for bs58PK, balance := range sheet.Anonymous {
		entry.anonymous[bs58PK] = balance
	}

	currentState = entry

	return entry, nil
}
