package ngstate

import (
	"strings"

	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
)

// isReservedKey guards the "_"-prefixed context keys (e.g. the lock flag)
// against contract access
func isReservedKey(key string) bool {
	return strings.HasPrefix(key, "_")
}

// vmContext resolves the journaled context of the account executing
// right now (the top call frame): a service callee gets ITS OWN storage
func vmContext(vm *VM) *ngtypes.AccountContext {
	ctx, err := vm.journal.contextOf(vm.txn, vm.currentAccount())
	if err != nil {
		vm.logger.Error(err)
		panic(err) // unreachable for loaded frames; abort the call
	}

	return ctx
}

// initKVImports binds the kv module: the contract's persistent key-value
// storage, backed by the EXECUTING account's Context. All writes go
// through the journal and get discarded when the call fails
func initKVImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("kv", "get_size", func(ins *wasman.Instance) interface{} {
		return func(keyPtr, keyLen uint32) uint32 {
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
			key, err := readMem(ins, keyPtr, keyLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if isReservedKey(string(key)) {
				return 0
			}

			val, err := readMem(ins, valPtr, valLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			value := make([]byte, len(val))
			copy(value, val)
			vmContext(vm).Set(string(key), value)

			return 1
		}
	})
	if err != nil {
		return err
	}

	// prefix iteration over the executing account's context: keys are
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
			key, err := readMem(ins, keyPtr, keyLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if isReservedKey(string(key)) {
				return 0
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
