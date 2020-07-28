package vm

import (
	"sync"

	"github.com/bytecodealliance/wasmtime-go"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("wasm")

var engine = wasmtime.NewEngine()

// WasmVM is a vm based on wasmer, exec wasm commands
// TODO: Update me after experiment WASI tests
type WasmVM struct {
	sync.RWMutex

	store    *wasmtime.Store
	module   *wasmtime.Module
	instance *wasmtime.Instance

	context []byte // the liner memory
}

// NewWasmVM creates a new Wasm
func NewWasmVM(contract, context []byte) (*WasmVM, error) {
	store := wasmtime.NewStore(engine)
	module, err := wasmtime.NewModule(engine, contract)
	if err != nil {
		return nil, err
	}

	return &WasmVM{
		store:    store,
		module:   module,
		instance: nil,
		context:  context,
	}, nil
}

func (vm *WasmVM) GetStore() *wasmtime.Store {
	vm.RLock()
	store := vm.store
	vm.RUnlock()

	return store
}

func (vm *WasmVM) GetModule() *wasmtime.Module {
	vm.RLock()
	module := vm.module
	vm.RUnlock()

	return module
}

const MaxLen = 1 << 32 // 2GB

func (vm *WasmVM) Instantiate(imports ...*wasmtime.Extern) error {
	instance, err := wasmtime.NewInstance(vm.store, vm.module, imports)
	if err != nil {
		return err
	}

	vm.Lock()

	vm.instance = instance

	// init context
	mem := vm.instance.GetExport("memory").Memory()
	length := mem.DataSize()
	if length >= MaxLen {
		length = MaxLen // avoid panic
	}
	vm.context = (*[MaxLen]byte)(mem.Data())[:length:length] // UnsafeData()

	vm.Unlock()
	return nil
}

func (vm *WasmVM) Start() {
	vm.RLock()
	defer vm.RUnlock()

	start := vm.instance.GetExport("_start") // run the wasm's main func, _start is same to WASI's main
	_, err := start.Func().Call()
	if err != nil {
		log.Error(err)
	}
}
