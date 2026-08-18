package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
)

// readAddr reads a 32-byte address out of the instance's linear memory
func readAddr(ins *wasman.Instance, ptr uint32) (ngtypes.Address, error) {
	raw, err := readMem(ins, ptr, uint32(ngtypes.AddressSize))
	if err != nil {
		return ngtypes.Address{}, err
	}

	addr := ngtypes.Address{}
	copy(addr[:], raw)
	return addr, nil
}

// initAddressImports binds the `address` host module: identity only —
// who is executing (get_host) and who called (get_caller). Contract
// introspection (is_active, code, code_hash) lives in the `contract`
// module. Addresses are plain 32-byte values passed through linear memory
func initAddressImports(vm *VM) error {
	err := vm.linker.DefineAdvancedFunc("address", "get_size", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(ngtypes.AddressSize)
		}
	})
	if err != nil {
		return err
	}

	// get_host writes the address whose code is executing right now:
	// for a service call this is the CALLEE
	err = vm.linker.DefineAdvancedFunc("address", "get_host", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			addr := vm.currentAddress()
			l, err := cp(ins, ptr, addr[:])
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

	// get_caller writes msg.from: the contract which invoked the
	// current frame; the zero address for the outermost frame
	err = vm.linker.DefineAdvancedFunc("address", "get_caller", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32) uint32 {
			addr := vm.callerAddress()
			l, err := cp(ins, ptr, addr[:])
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
