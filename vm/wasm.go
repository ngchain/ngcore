package vm

import (
	"github.com/wasmerio/go-ext-wasm/wasmer"
)

// WasmVM is a vm based on wasmer, exec wasm commands
type WasmVM struct {
	*wasmer.Instance
}

// NewWasmVM creates a new Wasm
func NewWasmVM(raw []byte) (*WasmVM, error) {
	instance, err := wasmer.NewInstance(raw)
	if err != nil {
		return nil, err
	}
	return &WasmVM{
		Instance: &instance,
	}, nil
}

// RunDeploy will run the main function of wasm. usually used for testing
func (vm *WasmVM) RunDeploy(ifaces ...interface{}) (wasmer.Value, error) {
	return vm.Exports["deploy"](ifaces...)
}
