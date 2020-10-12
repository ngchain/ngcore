package ngstate

import (
	"github.com/c0mm4nd/wasman"
	"reflect"
	"unsafe"
)

// InitBuiltInImports will bind go's host func with the contract module
func (vm *VM) InitBuiltInImports() error {
	err := initLogImports(vm)
	if err != nil {
		return err
	}

	err = initSelfImports(vm)
	if err != nil {
		return err
	}

	err = initCoinImports(vm)
	if err != nil {
		return err
	}

	return nil
}

func initLogImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("log", "debug", func(ins *wasman.Instance) interface{} {
		return func(ptr int32, l int32) {
			message := *(*string)(unsafe.Pointer(&reflect.StringHeader{
				Data: uintptr(ptr),
				Len:  int(l),
			}))
			vm.logger.Debug(message)
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func initSelfImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("self", "get_num", func(ins *wasman.Instance) interface{} {
		return func() int64 {
			return int64(vm.self.Num)
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("self", "get_owner", func(ins *wasman.Instance) interface{} {
		return func() int64 {
			return int64(vm.self.Num)
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
			err := vmTransfer(vm.txn, vm.self.Num, uint64(to), uint64(value))
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
