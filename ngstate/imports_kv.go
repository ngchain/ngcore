package ngstate

import (
	"strings"

	"github.com/c0mm4nd/wasman"
)

// isReservedKey guards the "_"-prefixed context keys (e.g. the lock flag)
// against contract access
func isReservedKey(key string) bool {
	return strings.HasPrefix(key, "_")
}

// initKVImports binds the kv module: the contract's persistent key-value
// storage, backed by its own account Context. All writes go through the
// journal and get discarded when the call fails
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

			return uint32(len(vm.journal.context.Get(string(key))))
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

			l, err := cp(ins, valPtr, vm.journal.context.Get(string(key)))
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
			vm.journal.context.Set(string(key), value)

			return 1
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

			vm.journal.context.Del(string(key))

			return 1
		}
	})
	if err != nil {
		return err
	}

	return nil
}
