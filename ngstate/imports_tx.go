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

	err = vm.linker.DefineAdvancedFunc("tx", "get_convener", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			return uint64(vm.caller.Convener)
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_participants_count", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(vm.caller.Participants))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_participant_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(ngtypes.Address{}))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_participant", func(ins *wasman.Instance) interface{} {
		return func(i, ptr uint32) uint32 {
			if i >= uint32(len(vm.caller.Participants)) {
				return 0
			}

			l, err := cp(ins, ptr, vm.caller.Participants[i][:])
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_value_size", func(ins *wasman.Instance) interface{} {
		return func(i uint32) uint32 {
			if i >= uint32(len(vm.caller.Values)) {
				return 0
			}

			return uint32(len(vm.caller.Values[i].Bytes()))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_value", func(ins *wasman.Instance) interface{} {
		return func(i, ptr uint32) uint32 {
			if i >= uint32(len(vm.caller.Values)) {
				return 0
			}

			l, err := cp(ins, ptr, vm.caller.Values[i].Bytes())
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_extra_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(len(vm.caller.Extra))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_extra", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, vm.caller.Extra)
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
