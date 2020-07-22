package ngstate

import (
	"github.com/ngchain/ngcore/storage"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type Manager struct {
	sync.RWMutex

	staticState *State // frozen state
	activeState *State // the state handling txs in realtime
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
	manager = initFromSheet(ngtypes.GenesisSheet, ngtypes.GetGenesisBlockHash())
	err := manager.UpgradeState(ngtypes.GetGenesisBlock())
	if err != nil {
		panic(err)
	}
}

// UpgradeState will create a new state which is a wrapper of *ngtypes.sheet
func initFromSheet(sheet *ngtypes.Sheet, newBlockHash []byte) *Manager {
	state := &State{
		prevBlockHash: newBlockHash,
		accounts:      make(map[uint64][]byte),
		anonymous:     make(map[string][]byte),
		pool:          NewTxPool(),
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

	staticState := state // static one
	activeState := state // active one

	return &Manager{
		staticState: staticState,
		activeState: activeState,
	}
}

func (m *Manager) GetStaticState() *State {
	m.RLock()
	defer m.RUnlock()

	return m.staticState
}

func (m *Manager) GetCurrentState() *State {
	m.RLock()
	defer m.RUnlock()

	return m.activeState
}

// UpgradeState will generate a new state after importing block's txs
func (m *Manager) UpgradeState(block *ngtypes.Block) error {
	m.Lock()
	defer m.Unlock()

	newState := &State{
		prevBlockHash: block.Hash(),
		accounts:      m.staticState.accounts,
		anonymous:     m.staticState.anonymous, // if copy it will cost too much time
		pool:          NewTxPool(),
	}
	err := newState.HandleTxs(block.Txs...)
	if err != nil {
		return err
	}

	m.staticState = newState // static one, for fallback and chain update
	m.activeState = newState // active one, for verification

	return nil
}

func (m *Manager) RegenerateState() error {
	m.Lock()
	defer m.Unlock()

	chain := storage.GetChain()
	newState := &State{
		prevBlockHash: ngtypes.GetGenesisBlockHash(),
		accounts:      make(map[uint64][]byte),
		anonymous:     make(map[string][]byte),
		pool:          NewTxPool(),
	}
	latest := chain.GetLatestBlockHeight()
	for h := uint64(0); h <= latest; h++ {
		block, _ := chain.GetBlockByHeight(h)
		err := newState.HandleTxs(block.Txs...)
		if err != nil {
			return err
		}
	}
	m.staticState = newState // static one, for fallback and chain update
	m.activeState = newState // active one, for verification

	return nil
}

// DowngradeState will generate a new state after importing block's txs
//func (m *Manager) DowngradeState(block *ngtypes.Block) error {
//	m.Lock()
//	defer m.Unlock()
//
//	newState := &State{
//		prevBlockHash: block.Hash(),
//		accounts:      m.staticState.accounts,
//		anonymous:     m.staticState.anonymous, // if copy it will cost too much time
//		pool:          NewTxPool(),
//	}
//	err := newState.HandleTxs(block.Txs...)
//	if err != nil {
//		return err
//	}
//
//	m.staticState = newState // static one, for fallback and chain update
//	m.activeState = newState // active one, for verification
//
//	return nil
//}
