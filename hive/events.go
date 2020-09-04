package hive

import (
	"github.com/ngchain/ngcore/ngtypes"
)

// call me when applying new tx
func (vm *VM) Call(tx *ngtypes.Tx) {
	vm.RLock()
	defer vm.RUnlock()

	export := vm.instance.GetExport("main") // main's params should be ( i64)
	if export == nil {
		return
	}

	f := export.Func()
	if f == nil {
		return
	}

	ok, err := f.Call(tx)
	if err != nil {
		vm.logger.Error(err)
	}

	if !(ok.(bool)) {
		vm.logger.Error("failed on call")
	}
}
