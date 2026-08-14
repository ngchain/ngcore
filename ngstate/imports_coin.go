package ngstate

import (
	"github.com/c0mm4nd/wasman"
)

func initCoinImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("coin", "get_balance_size", func(ins *wasman.Instance) interface{} {
		return func(addrPtr uint32) uint32 {
			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(vm.journal.balanceOf(vm.txn, addr).Bytes()))
		}
	})
	if err != nil {
		return err
	}

	// get_balance writes the big-endian bytes of the balance into ptr
	err = vm.linker.DefineAdvancedFunc("coin", "get_balance", func(ins *wasman.Instance) interface{} {
		return func(addrPtr, ptr uint32) uint32 {
			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, vm.journal.balanceOf(vm.txn, addr).Bytes())
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

	// transfer moves value from the EXECUTING address to the `to`
	// address, through the journal: nothing is final until the whole
	// call succeeds. Within a service call the callee spends its own
	// funds, never the caller's
	err = vm.linker.DefineAdvancedFunc("coin", "transfer", func(ins *wasman.Instance) interface{} {
		return func(toPtr uint32, value uint64) uint32 {
			vm.charge(gasCoinTransfer)

			to, err := readAddr(ins, toPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			err = vm.journal.transfer(vm.txn, vm.currentAccount(), to, bigIntFromUint64(value))
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
