package vm

import (
	"github.com/ngchain/ngcore/ngtypes"
)

// Manager is a manager to control the life circle of state vms
// TODO
type Manager struct {
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
	vm, err := NewWasmVM(account.Contract)
	if err != nil {
		return nil, err
	}

	m.vms[account.Num] = vm

	return vm, nil
}
