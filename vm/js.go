package vm

import (
	"github.com/dop251/goja"
)

type JsVM struct {
	*goja.Runtime
}

func NewJsVM() *JsVM {
	runtime := goja.New()
	return &JsVM{
		Runtime: runtime,
	}
}

func (vm *JsVM) RunState(raw []byte) []byte {
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
