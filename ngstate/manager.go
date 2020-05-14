package ngstate

import (
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("sheet")

var state *State

// GetCurrentState will create a Sheet manager
func GetCurrentState() *State {
	if state == nil {
		panic("failed to get current state from nil")
	}

	return state
}
