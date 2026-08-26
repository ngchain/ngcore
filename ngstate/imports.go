package ngstate

import (
	"strings"

	"github.com/c0mm4nd/wasman"
)

// initBuiltInImports binds all host modules to the contract linker:
// log, address (identity), coin, kv, tx, env (buf slots + gas), crypto,
// and contract (dynamic call + introspection). The wideint u128/u256
// modules come from wasman itself (EnableWideInt). The host ABI is
// specified at paper.ngchain.org; the imports_*.go files here are the
// canonical implementation.
func (vm *VM) initBuiltInImports() error {
	for _, init := range []func(*VM) error{
		initLogImports,
		initAddressImports,
		initCoinImports,
		initKVImports,
		initTxImports,
		initEnvImports,
		initCryptoImports,
		initContractImports,
	} {
		if err := init(vm); err != nil {
			return err
		}
	}

	return nil
}

func initLogImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("log", "debug", func(ins *wasman.Instance) interface{} {
		return func(ptr, size uint32) {
			message, err := readMem(ins, ptr, size)
			if err != nil {
				vm.logger.Error(err)
				return
			}

			vm.logger.Debug(string(message))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("log", "error", func(ins *wasman.Instance) interface{} {
		return func(ptr, size uint32) {
			message, err := readMem(ins, ptr, size)
			if err != nil {
				vm.logger.Error(err)
				return
			}

			vm.logger.Error(string(message))
		}
	})
	if err != nil {
		return err
	}

	// emit records a contract event into the tx receipt (local,
	// non-consensus data); attributed to the EXECUTING address
	err = vm.linker.DefineAdvancedFunc("log", "emit", func(ins *wasman.Instance) interface{} {
		return func(topicPtr, topicLen, dataPtr, dataLen uint32) uint32 {
			vm.charge(gasEventBase + gasEventPerByte*uint64(topicLen+dataLen))

			if len(vm.events) >= maxEventsPerRun ||
				topicLen > maxEventTopicLen || dataLen > maxEventDataLen {
				return 0
			}

			topic, err := readMem(ins, topicPtr, topicLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			// the "ng." topic namespace is reserved for node-emitted system
			// logs (e.g. internal transfers), so those stay unforgeable
			if strings.HasPrefix(string(topic), EventTopicPrefix) {
				return 0
			}
			data, err := readMem(ins, dataPtr, dataLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			vm.events = append(vm.events, Event{
				Contract: vm.currentAddress().Bytes(),
				Topic:    string(topic),
				Data:     dataCopy,
			})

			return 1
		}
	})
	if err != nil {
		return err
	}

	return nil
}

// initEnvImports binds the env module: execution-environment
// introspection shared by the whole call tree
func initEnvImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("env", "get_gas", func(ins *wasman.Instance) interface{} {
		return func() uint64 {
			// the remaining toll budget of this call tree
			spent := vm.cfg.TollStation.GetToll()
			if spent >= vmMaxToll {
				return 0
			}

			return vmMaxToll - spent
		}
	})
	if err != nil {
		return err
	}

	// the transfer slots ferry byte payloads across service frames:
	// instances have separate linear memories, so the caller stages
	// bytes into a slot before the call and the callee reads them out
	// (and writes results back the same way)
	err = vm.linker.DefineAdvancedFunc("env", "buf_set", func(ins *wasman.Instance) interface{} {
		return func(slot, ptr, size uint32) uint32 {
			if slot >= vmBufSlots || size > vmBufMaxLen {
				return 0
			}

			data, err := readMem(ins, ptr, size)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			buf := make([]byte, len(data))
			copy(buf, data)
			vm.bufs[slot] = buf

			return 1
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("env", "buf_size", func(ins *wasman.Instance) interface{} {
		return func(slot uint32) uint32 {
			if slot >= vmBufSlots {
				return 0
			}

			return uint32(len(vm.bufs[slot]))
		}
	})
	if err != nil {
		return err
	}

	err = vm.linker.DefineAdvancedFunc("env", "buf_get", func(ins *wasman.Instance) interface{} {
		return func(slot, ptr uint32) uint32 {
			if slot >= vmBufSlots {
				return 0
			}

			l, err := cp(ins, ptr, vm.bufs[slot])
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
