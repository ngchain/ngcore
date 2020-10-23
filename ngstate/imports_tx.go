package ngstate

import "C"
import (
	"github.com/c0mm4nd/wasman"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
)

func initTxImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("tx", "get_caller_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return ngtypes.HashSize // caller is the hash of tx
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_caller", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			l, err := cp(ins, ptr, vm.caller.Hash())
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_prev_hash_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return ngtypes.HashSize // caller is the hash of tx
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_prev_hash", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, ptr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, tx.PrevBlockHash)
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_convener", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint64 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return tx.Convener
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_signature_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return ngtypes.SignatureSize // 64
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_signature", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, ptr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, tx.Sign)
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
		return func(hashPtr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(tx.Extra))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_extra", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, ptr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, tx.Extra)
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
		return func(hashPtr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(tx.Fee))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_fee", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, ptr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, tx.Fee)
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
		return func(hashPtr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(tx.Participants))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_participant_size", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, i uint32) uint32 {
			return ngtypes.AddressSize
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_participant", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, i uint32, ptr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, tx.Participants[i])
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
		return func(hashPtr uint32, i uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(tx.Values[i]))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("tx", "get_value", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, i uint32, ptr uint32) uint32 {
			rawTxHash := ins.Memory.Value[hashPtr : hashPtr+ngtypes.HashSize]

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, ptr, tx.Values[i])
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
