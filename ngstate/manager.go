package ngstate

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type Manager struct {
	prevSheetHash []byte
	prevState     *State // frozen state
	CurrentState  *State // the state handling txs in realtime
}

// (nil) --> B0(Prev: S0) --> B1(Prev: S1) -> B2(Prev: S2)
//  init (S0,S0)  -->   (S0,S1)  -->    (S1, S2)
var manager *Manager // one instance

func GetStateManager() *Manager {
	if manager == nil {
		panic("manager is nil")
	}

	return manager
}

func init() {
	manager = initFromSheet(ngtypes.GenesisSheet)
	manager.UpdateState(ngtypes.GetGenesisBlock())
}

// UpdateState will create a new state which is a wrapper of *ngtypes.sheet
func initFromSheet(sheet *ngtypes.Sheet) *Manager {
	state := &State{
		prevSheetkHash: sheet.Hash(),
		height:         sheet.Height + 1,
		accounts:       make(map[uint64][]byte),
		anonymous:      make(map[string][]byte),
		pool: &TxPool{
			txs: make([]*ngtypes.Tx, 0),
		},
	}

	var err error
	for id, account := range sheet.Accounts {
		state.accounts[id], err = utils.Proto.Marshal(account)
		if err != nil {
			panic(err)
		}
	}

	for bs58PK, balance := range sheet.Anonymous {
		state.anonymous[bs58PK] = balance
	}

	prevSheetHash := sheet.Hash()
	prevState := state    // static one
	currentState := state // active one

	return &Manager{
		prevSheetHash: prevSheetHash,
		prevState:     prevState,
		CurrentState:  currentState,
	}
}

func (m *Manager) UpdateState(block *ngtypes.Block) error {
	if !bytes.Equal(block.PrevSheetHash, m.prevSheetHash) {
		return fmt.Errorf("the new block doesnt belong to local chain")
	}

	newState := m.prevState
	err := newState.ApplyTxs(block.Txs...)
	if err != nil {
		return err
	}

	sheet := newState.ToSheet()
	m.prevSheetHash = sheet.Hash()
	m.prevState = newState    // static one
	m.CurrentState = newState // active one

	return nil
}
