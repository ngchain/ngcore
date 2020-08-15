package vm

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/ngtypes"
)

const defaultHTTPVMServerPort = 52528 // TODO: move this into app flag

// Manager is a manager to control the life circle of state vms
// TODO: Update me after experiment WASI tests
type Manager struct {
	sync.RWMutex

	server http.Server      // for frontend app
	engine *wasmtime.Engine // for backend app

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

	m.Lock()
	m.vms[account.Num] = vm
	m.Unlock()

	return vm, nil
}

func (m *Manager) GetVM(num uint64) (*WasmVM, error) {
	m.RLock()
	vm, exists := m.vms[num]
	m.RUnlock()
	if !exists {
		return nil, fmt.Errorf("vm is not exists")
	}

	return vm, nil
}
