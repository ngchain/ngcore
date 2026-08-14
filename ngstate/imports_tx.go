package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
)

// initTxImports binds the tx module, which exposes the calling tx —
// the one which triggered this contract execution
func initTxImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("tx", "get_hash_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return ngtypes.HashSize
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_hash", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, vm.caller.GetHash())
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_network", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(vm.caller.Network)
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_height", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			return vm.caller.Height
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_timestamp", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			// the enclosing block's timestamp (unix seconds)
			return vm.blockTime
		}
	})
	if err != nil {
		return err
	}

	// get_paid exposes msg.value: what this tx pays to the address
	// executing right now, as big-endian big.Int bytes (zero for any
	// frame other than the tx's To address)
	paidToCurrent := func() []byte {
		if vm.caller.To != vm.currentAddress() {
			return nil
		}

		return vm.caller.Value.Bytes()
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_paid_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(paidToCurrent()))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_paid", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, paidToCurrent())
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

	// get_from writes the tx's From address (derived from the
	// signature envelope); zero address when the tx is unsigned
	err = vm.linker.DefineAdvancedFunc("tx", "get_from", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			from, err := vm.caller.From()
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, from[:])
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

	// get_to writes the tx's To address
	err = vm.linker.DefineAdvancedFunc("tx", "get_to", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, vm.caller.To[:])
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_fee_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(vm.caller.Fee.Bytes()))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_fee", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, vm.caller.Fee.Bytes())
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

	// get_extra serves the ARGS part of the calldata: the entry
	// selector is routing information already consumed by the runtime
	err = vm.linker.DefineAdvancedFunc("tx", "get_extra_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(vm.callArgs))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_extra", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, vm.callArgs)
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

	return nil
}
