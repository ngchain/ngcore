package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
)

func initAccountImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("account", "get_host", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			return vm.self.Num // host means the account which is hosting the contract
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_owner_size", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			return ngtypes.AddressSize // addr is 35 bytes
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

			l, err := cp(ins, ptr, acc.Owner)
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

	err = vm.linker.DefineAdvancedFunc("account", "get_context_size", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(acc.Context))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("account", "get_context", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64, ptr uint32) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, acc.Context)
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

	//TODO:  write to Context when num is self
	//err = vm.linker.DefineAdvancedFunc("account", "write_context", func(ins *wasman.Instance) interface{} {
	//	return func(accountNum uint64, ptr uint32) uint32 {
	//		acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
	//		if err != nil {
	//			vm.logger.Error(err)
	//			return 0
	//		}
	//
	//		l, err := cp(ins, ptr, acc.Context)
	//		if err != nil {
	//			vm.logger.Error(err)
	//			return 0
	//		}
	//
	//		return l
	//	}
	//})
	//if err != nil {
	//	return err
	//}

	return nil
}
