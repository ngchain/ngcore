package vm

import (
	"github.com/dop251/goja"
)

// JSVM is a javascript VM for exec javascript codes from account state, based on goja
type JSVM struct {
	*goja.Runtime
}

// NewJSVM is a new javascript VM
func NewJSVM() *JSVM {
	runtime := goja.New()
	return &JSVM{
		Runtime: runtime,
	}
}

// RunState runs the state from account
func (vm *JSVM) RunState(raw []byte) []byte {
	_, err := vm.RunScript("contract", string(raw))
	if err != nil {
		log.Error(err)
		return nil
	}

	result, err := vm.RunString(`new Contract().main();`)
	if err != nil {
		log.Error(err)
		return nil
	}

	return []byte(result.String())
}

// Watch is used to update tx extra code from a specific account
// TODO
func (vm *JSVM) Watch(accountID uint64) {

}
