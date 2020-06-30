package ngstate

import (
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type Manager struct {
	sync.RWMutex

	prevSheetHash []byte
	prevState     *State // frozen state
	currentState  *State // the state handling txs in realtime
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
		prevSheetHash: sheet.Hash(),
		height:        sheet.Height + 1,
		accounts:      make(map[uint64][]byte),
		anonymous:     make(map[string][]byte),
		pool: &TxPool{
			txMap: make(map[uint64]*ngtypes.Tx, 0),
		},
	}

	var err error
	for id, account := range sheet.Accounts {
		state.accounts[id], err = utils.Proto.Marshal(account)
		if err != nil {
			panic(err)
		}
	}

	for bs58Address, balance := range sheet.Anonymous {
		state.anonymous[bs58Address] = balance
	}

	prevSheetHash := sheet.Hash()
	prevState := state    // static one
	currentState := state // active one

	return &Manager{
		prevSheetHash: prevSheetHash,
		prevState:     prevState,
		currentState:  currentState,
	}
}

func (m *Manager) UpdateState(block *ngtypes.Block) error {
	m.Lock()
	defer m.Unlock()

	newState := m.prevState
	err := newState.HandleTxs(block.Txs...)
	if err != nil {
		return err
	}

	sheet := newState.ToSheet()
	m.prevSheetHash = sheet.Hash()
	m.prevState = newState    // static one, for fallback and chain update
	m.currentState = newState // active one, for verification

	return nil
}

func (m *Manager) GetPrevState() *State {
	m.RLock()
	defer m.RUnlock()

	return m.prevState
}

func (m *Manager) GetCurrentState() *State {
	m.RLock()
	defer m.RUnlock()

	return m.currentState
}
