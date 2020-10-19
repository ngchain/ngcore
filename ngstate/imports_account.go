package ngstate

import (
	"github.com/c0mm4nd/wasman"
	"github.com/ngchain/ngcore/ngtypes"
)

func initAccountImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("account", "get_self", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			return vm.self.Num
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_owner", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, acc.Owner)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return ptr
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_contract", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, acc.Contract)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return ptr
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_context", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, acc.Context)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return ptr
		}
	})
	if err != nil {
		return err
	}

	//TODO: write to Acc.Context when num is self

	return nil
}
