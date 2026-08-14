package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
)

func initCoinImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("coin", "get_balance_size", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(vm.journal.balanceOf(vm.txn, acc.Owner).Bytes()))
		}
	})
	if err != nil {
		return err
	}

	// get_balance writes the big-endian bytes of the balance into ptr
	err = vm.linker.DefineAdvancedFunc("coin", "get_balance", func(ins *wasman.Instance) interface{} {
		return func(accountNum uint64, ptr uint32) uint32 {
			acc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(accountNum))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, vm.journal.balanceOf(vm.txn, acc.Owner).Bytes())
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

	// transfer moves value from the EXECUTING account to the `to`
	// account, through the journal: nothing is final until the whole
	// call succeeds. Within a service call the callee spends its own
	// funds, never the caller's
	err = vm.linker.DefineAdvancedFunc("coin", "transfer", func(ins *wasman.Instance) interface{} {
		return func(to, value uint64) uint32 {
			fromAcc, err := vm.journal.accountOf(vm.txn, vm.currentAccount())
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			toAcc, err := getAccountByNum(vm.txn, ngtypes.AccountNum(to))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			err = vm.journal.transfer(vm.txn, fromAcc.Owner, toAcc.Owner, bigIntFromUint64(value))
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
