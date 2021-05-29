package ngstate

import (
	"github.com/c0mm4nd/wasman"
)

// InitBuiltInImports will bind go's host func with the contract module
func (vm *VM) InitBuiltInImports() error {
	err := initLogImports(vm)
	if err != nil {
		return err
	}

	err = initAccountImports(vm)
	if err != nil {
		return err
	}

	err = initCoinImports(vm)
	if err != nil {
		return err
	}

	err = initTxImports(vm)
	if err != nil {
		return err
	}

	return nil
}

func initLogImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("log", "debug", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32, size uint32) {
			message := string(ins.Memory.Value[ptr : ptr+size])
			// TODO: turn off debug by default
			// RULE: add --vm-debug <AccountNum1>,<AccountNum2> to enable debug
			vm.logger.Debug(message)
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func initCoinImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("coin", "transfer", func(ins *wasman.Instance) interface{} {
		return func(to, value int64) int32 {
			err := vmTransfer(vm.txn, vm.self.Proto.GetNum(), uint64(to), uint64(value))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return 1
		}
	})
	if err != nil {
		return err
	}

	return nil
}
