package ngstate

import (
	"math/big"

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

	// get_paid exposes msg.value: the total this tx pays to the address
	// executing right now, as big-endian big.Int bytes
	paidToCurrent := func() []byte {
		current := vm.currentAddress()

		total := new(big.Int)
		for i := range vm.caller.Participants {
			if i < len(vm.caller.Values) && vm.caller.Participants[i] == current {
				total.Add(total, vm.caller.Values[i])
			}
		}

		return total.Bytes()
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

	// get_sender writes the tx sender's address (derived from the
	// signature envelope); zero address when the tx is unsigned
	err = vm.linker.DefineAdvancedFunc("tx", "get_sender", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			sender, err := vm.caller.Sender()
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, sender[:])
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
