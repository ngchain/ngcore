package ngstate

import (
	"strings"

	"github.com/c0mm4nd/wasman"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// isReservedKey guards the "_"-prefixed context keys (_deps, _refs, the lock
// flag) against contract access. Those are all SHORT names; a contract
// legitimately stores data keyed by a 32-byte address, and ~1/256 of
// addresses start with '_' (0x5f) — so an address-LENGTH key is never
// reserved, otherwise those addresses would be unusable as storage keys.
func isReservedKey(key string) bool {
	return strings.HasPrefix(key, "_") && len(key) != ngtypes.AddressSize
}

// vmContext resolves the journaled context of the address executing
// right now (the top call frame): a service callee gets ITS OWN storage
func vmContext(vm *VM) *ngtypes.ContractContext {
	ctx, err := vm.journal.contextOf(vm.txn, vm.currentAddress())
	if err != nil {
		vm.logger.Error(err)
		panic(err) // unreachable for loaded frames; abort the call
	}

	return ctx
}

// initKVImports binds the kv module: the contract's persistent key-value
// storage, backed by the EXECUTING address's Context. All writes go
// through the journal and get discarded when the call fails
func initKVImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("kv", "get_size", func(ins *wasman.Instance) interface{} {
		return func(keyPtr, keyLen uint32) uint32 {
			vm.charge(gasKVRead)

			key, err := readMem(ins, keyPtr, keyLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if isReservedKey(string(key)) {
				return 0
			}

			return uint32(len(vmContext(vm).Get(string(key))))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("kv", "get", func(ins *wasman.Instance) interface{} {
		return func(keyPtr, keyLen, valPtr uint32) uint32 {
			vm.charge(gasKVRead)

			key, err := readMem(ins, keyPtr, keyLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if isReservedKey(string(key)) {
				return 0
			}

			l, err := cp(ins, valPtr, vmContext(vm).Get(string(key)))
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

	err = vm.linker.DefineAdvancedFunc("kv", "set", func(ins *wasman.Instance) interface{} {
		return func(keyPtr, keyLen, valPtr, valLen uint32) uint32 {
			vm.charge(gasKVSetBase + gasKVSetPerByte*uint64(keyLen+valLen))

			key, err := readMem(ins, keyPtr, keyLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if isReservedKey(string(key)) {
				// writes to reserved keys trap LOUDLY: silently dropping
				// them turns an authoring bug into hours of debugging.
				// (Reads stay quiet — probing is harmless.)
				panic(errors.Errorf("kv.set on reserved key %q", key))
			}

			val, err := readMem(ins, valPtr, valLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			value := make([]byte, len(val))
			copy(value, val)

			// storage deposit (ForkStateRent): the deposit a contract owes is a
			// pure function of the bytes it stores, so a write only moves the
			// DELTA. Grow -> lock the added bytes' bond from the contract's own
			// balance into the escrow; shrink -> refund the removed bytes' bond.
			// Charged AFTER the reserved-key trap and gas charge, BEFORE the
			// Set. Pre-fork this whole block is skipped: gas-only, exactly as
			// before. The move is journaled, so a later trap rolls it back.
			if ngtypes.IsForkActive(vm.caller.Network, ngtypes.ForkStateRent, vm.caller.Height) {
				ctx := vmContext(vm)
				old := 0
				if ctx.Has(string(key)) { // present (even with an empty value) owes key+value
					old = len(key) + len(ctx.Get(string(key)))
				}
				newSize := len(key) + len(value)
				if delta := newSize - old; delta > 0 {
					if err := vm.lockDeposit(depositFor(delta)); err != nil {
						panic(err) // insufficient balance for storage deposit -> soft-fail
					}
				} else if delta < 0 {
					vm.refundDeposit(depositFor(-delta))
				}
			}

			vmContext(vm).Set(string(key), value)

			return 1
		}
	})
	if err != nil {
		return err
	}

	// prefix iteration over the executing address's context: keys are
	// canonically sorted, so index-based access is deterministic.
	// Reserved ("_"-prefixed) keys stay invisible
	matchingKeys := func(prefix []byte) []string {
		ctx := vmContext(vm)
		keys := make([]string, 0)
		for _, k := range ctx.Keys {
			if isReservedKey(k) {
				continue
			}
			if len(k) >= len(prefix) && k[:len(prefix)] == string(prefix) {
				keys = append(keys, k)
			}
		}
		return keys
	}

	err = vm.linker.DefineAdvancedFunc("kv", "count", func(ins *wasman.Instance) interface{} {
		return func(prefixPtr, prefixLen uint32) uint32 {
			vm.charge(gasKVRead)

			prefix, err := readMem(ins, prefixPtr, prefixLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return uint32(len(matchingKeys(prefix)))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("kv", "key_size_at", func(ins *wasman.Instance) interface{} {
		return func(prefixPtr, prefixLen, index uint32) uint32 {
			vm.charge(gasKVRead)

			prefix, err := readMem(ins, prefixPtr, prefixLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			keys := matchingKeys(prefix)
			if index >= uint32(len(keys)) {
				return 0
			}

			return uint32(len(keys[index]))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("kv", "key_at", func(ins *wasman.Instance) interface{} {
		return func(prefixPtr, prefixLen, index, outPtr uint32) uint32 {
			vm.charge(gasKVRead)

			prefix, err := readMem(ins, prefixPtr, prefixLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			keys := matchingKeys(prefix)
			if index >= uint32(len(keys)) {
				return 0
			}

			l, err := cp(ins, outPtr, []byte(keys[index]))
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

	err = vm.linker.DefineAdvancedFunc("kv", "del", func(ins *wasman.Instance) interface{} {
		return func(keyPtr, keyLen uint32) uint32 {
			vm.charge(gasKVDel)

			key, err := readMem(ins, keyPtr, keyLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if isReservedKey(string(key)) {
				// a reserved-key delete is an authoring bug: trap loudly
				panic(errors.Errorf("kv.del on reserved key %q", key))
			}

			// storage deposit (ForkStateRent): deleting an entry frees its
			// whole bond back to the contract's balance from the escrow. An
			// absent key frees nothing. Journaled -> rolls back with a trap.
			// Pre-fork this is skipped: gas-only, exactly as before.
			if ngtypes.IsForkActive(vm.caller.Network, ngtypes.ForkStateRent, vm.caller.Height) {
				ctx := vmContext(vm)
				if ctx.Has(string(key)) {
					freed := len(key) + len(ctx.Get(string(key)))
					vm.refundDeposit(depositFor(freed))
				}
			}

			vmContext(vm).Del(string(key))

			return 1
		}
	})
	if err != nil {
		return err
	}

	return nil
}
