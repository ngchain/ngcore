package vm

import (
	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/ngtypes"
)

// Manager is a manager to control the life circle of state vms
// TODO: Update me after experiment WASI tests
type Manager struct {
	engine *wasmtime.Engine

	vms map[uint64]*WasmVM
}

// NewVMManager creates a new manager of wasm VM
func NewVMManager() *Manager {
	return &Manager{
		vms: map[uint64]*WasmVM{},
	}
}

// CreateVM creates a new wasm vm
func (m *Manager) CreateVM(account *ngtypes.Account) (*WasmVM, error) {
	vm, err := NewWasmVM(account.Contract, account.Context)
	if err != nil {
		return nil, err
	}

	m.vms[account.Num] = vm

	return vm, nil
}
