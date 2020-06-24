package ngstate_test

import (
	"testing"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestInitialState(t *testing.T) {
	state := ngstate.GetCurrentState()
	state.ToSheet()
	state.GetBalanceByNum(1)
	state.GetBalanceByAddress(ngtypes.GenesisAddress)
}

func TestTxPool(t *testing.T) {
	ngstate.GetCurrentState()
	ngstate.GetTxPool()
	// tx := ngtypes.NewUnsignedTx(ngtypes.TxType_TRANSACTION, ngtypes.GetGenesisBlockHash(), 1)
	// pool.PutTx(tx)
}
