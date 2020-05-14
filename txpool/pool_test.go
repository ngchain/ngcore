package txpool_test

import (
	"testing"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/txpool"
)

func TestNewTxPool(t *testing.T) {
	state := &ngstate.State{}
	pool := txpool.NewTxPool(state)
	pool.Init(nil)
}
