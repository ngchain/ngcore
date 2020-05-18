package ngstate

import (
	"sync"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("sheet")

type State struct {
	sync.RWMutex

	// using bytes to keep data safe
	accounts  map[uint64][]byte
	anonymous map[string][]byte
}

var state *State

// GetCurrentState will create a Sheet manager
func GetCurrentState() *State {
	if state == nil {
		panic("failed to get current state from nil")
	}

	return state
}

// NewStateFromSheet will create a new state which is a wrapper of *ngtypes.sheet
func NewStateFromSheet(sheet *ngtypes.Sheet) (*State, error) {
	entry := &State{
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

	state = entry

	return entry, nil
}
