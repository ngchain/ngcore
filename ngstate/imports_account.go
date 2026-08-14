package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
)

func initAccountImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("account", "get_host", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			// host is the account whose code is executing right now: for
			// a service call this is the CALLEE
			return vm.currentAccount()
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_caller", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			// msg.sender: the contract which invoked the current frame,
			// 0 for the outermost frame
			return vm.callerAccount()
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_owner_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(ngtypes.Address{}))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_owner", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64, ptr uint32) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, acc.Owner[:])
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return l
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_contract_size", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(acc.Contract))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_contract", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64, ptr uint32) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, acc.Contract)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return l
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "is_locked", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if acc.IsLocked() {
				return 1
			}

			return 0
		}
	})
	if err != nil {
		return err
	}

	return nil
}
