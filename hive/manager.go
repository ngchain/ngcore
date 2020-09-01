package hive

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/hive/wasm"
)

// temporarily stop HTTP wasm design -> avoid legal issue on copyright and uncensored media
// const defaultHTTPVMServerPort = 52528 // TODO: move this into app flag

var vmManager *Manager

// Manager is a manager to control the life circle of state vms
// TODO: Update me after experiment WASI tests
type Manager struct {
	sync.RWMutex

	server http.Server      // for frontend app
	engine *wasmtime.Engine // for backend app

	srcs map[uint64][]byte
	vms  map[uint64]*wasm.VM
}

// InitVMManager creates a new manager of wasm VM
func InitVMManager() {
	vmManager = &Manager{
		vms: map[uint64]*wasm.VM{},
	}
}

// CreateVM creates a new wasm vm
func CreateVM(num uint64, contract []byte) (*wasm.VM, error) {
	vm, err := wasm.NewVM(num, contract)
	if err != nil {
		return nil, err
	}

	vmManager.Lock()
	vmManager.vms[num] = vm
	vmManager.Unlock()

	return vm, nil
}

func GetVM(num uint64) (*wasm.VM, error) {
	vmManager.RLock()
	vm, exists := vmManager.vms[num]
	vmManager.RUnlock()
	if !exists {
		return nil, fmt.Errorf("vm is not exists")
	}

	return vm, nil
}
