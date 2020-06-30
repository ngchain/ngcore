package ngstate_test

import (
	"testing"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestInitialState(t *testing.T) {
	state := ngstate.GetActiveState()
	state.ToSheet()
	_, err := state.GetBalanceByNum(1)
	if err != nil {
		panic(err)
	}
	_, err = state.GetBalanceByAddress(ngtypes.GenesisAddress)
	if err != nil {
		panic(err)
	}
}

func TestTxPool(t *testing.T) {
	ngstate.GetActiveState()
	ngstate.GetActiveState().GetPool()
	// tx := ngtypes.NewUnsignedTx(ngtypes.TxType_TRANSACTION, ngtypes.GetGenesisBlockHash(), 1)
	// pool.PutTx(tx)
}
