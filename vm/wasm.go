package vm

import (
	"github.com/bytecodealliance/wasmtime-go"
)

var engine = wasmtime.NewEngine()

// WasmVM is a vm based on wasmer, exec wasm commands
// TODO: Update me after experiment WASI tests
type WasmVM struct {
	store      *wasmtime.Store
	mainModule *wasmtime.Module
	instance   *wasmtime.Instance
	context    []byte
}

// NewWasmVM creates a new Wasm
func NewWasmVM(contract, context []byte) (*WasmVM, error) {
	store := wasmtime.NewStore(engine)
	module, err := wasmtime.NewModule(store, contract)
	if err != nil {
		return nil, err
	}

	return &WasmVM{
		store:      store,
		mainModule: module,
		instance:   nil,
		context:    context,
	}, nil
}

func (vm *WasmVM) Instantiate() {
	wasi, err := wasmtime.NewWasiInstance(vm.store, getWASICfg(), "wasi_snapshot_preview1")
	if err != nil {
		panic(err)
	}

	linker := wasmtime.NewLinker(vm.store)
	err = linker.DefineWasi(wasi)
	if err != nil {
		panic(err)
	}

	instance, err := linker.Instantiate(vm.mainModule)
	if err != nil {
		panic(err)
	}

	vm.instance = instance
}

func (vm *WasmVM) Start() {
	start := vm.instance.GetExport("_start") // run the wasm's main func
	_, err := start.Func().Call()
	if err != nil {
		panic(err)
	}
}
