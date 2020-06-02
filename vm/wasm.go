package vm

import (
	"github.com/bytecodealliance/wasmtime-go"
)

var engine = wasmtime.NewEngine()

// WasmVM is a vm based on wasmer, exec wasm commands
// TODO: Update me after experiment WASI tests
type WasmVM struct {
	*wasmtime.Instance
}

// NewWasmVM creates a new Wasm
func NewWasmVM(rawContract []byte) (*WasmVM, error) {
	store := wasmtime.NewStore(engine)
	module, err := wasmtime.NewModule(store, rawContract)
	if err != nil {
		return nil, err
	}

	instance, err := wasmtime.NewInstance(module, nil)
	if err != nil {
		return nil, err
	}
	return &WasmVM{
		Instance: instance,
	}, nil
}

// RunDeploy will run the main function of wasm. usually used for testing
func (vm *WasmVM) RunDeploy(ifaces ...interface{}) (interface{}, error) {
	return vm.GetExport("deploy").Func().Call(ifaces...)
}
