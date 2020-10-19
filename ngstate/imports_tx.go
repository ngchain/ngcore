package ngstate

import "C"
import (
	"github.com/c0mm4nd/wasman"
	"github.com/ngchain/ngcore/ngblocks"
)

func initTxImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("tx", "get_caller", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			ptr, err := writeToMem(ins, vm.caller.Hash())
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_prev_hash", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, tx.PrevBlockHash)
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_convener", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint64 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

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

	err = vm.linker.DefineAdvancedFunc("tx", "get_signature", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, tx.Sign)
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_extra", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, tx.Extra)
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_fee", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, tx.Fee)
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_participants_num", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

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

	err = vm.linker.DefineAdvancedFunc("tx", "get_participant", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, i uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, tx.Participants[i])
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

	err = vm.linker.DefineAdvancedFunc("tx", "get_value", func(ins *wasman.Instance) interface{} {
		return func(hashPtr uint32, i uint32) uint32 {
			rawTxHash := []byte(strFromPtr(ins, hashPtr))

			tx, err := ngblocks.GetTxByHash(vm.txn, rawTxHash)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			ptr, err := writeToMem(ins, tx.Values[i])
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

	return nil
}
