package ngstate

import (
	"github.com/c0mm4nd/wasman"
	"github.com/ngchain/ngcore/ngtypes"
)

// Call when applying new tx
func (vm *VM) Call(ins *wasman.Instance, tx *ngtypes.Tx) {
	vm.RLock()
	defer vm.RUnlock()

	// TODO: add tx into the external modules

	_, _, err := ins.CallExportedFunc("main") // main's params should be ( i64)
	if err != nil {
		vm.logger.Error(err)
	}
}
