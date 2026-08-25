package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/utils"
)

// initContractImports binds the `contract` host module: the tools a
// contract uses to reach OTHER contracts — invoke one, and introspect a
// contract at an address (liveness, its code, the code hash). A static
// import names a fixed dependee by its bs58 address; `contract.call`
// dispatches to an address chosen at RUNTIME.
func initContractImports(vm *VM) error {
	// call(addr_ptr, args_ptr, args_len) -> 1|0: invoke the contract at
	// addr on ITS OWN state, passing calldata, 1 on success. The address
	// is resolved at runtime (vs a static import fixed at lock time)
	err := vm.linker.DefineAdvancedFunc("contract", "call", func(ins *wasman.Instance) interface{} {
		return func(addrPtr, argsPtr, argsLen uint32) uint32 {
			return vm.serviceCall(ins, addrPtr, argsPtr, argsLen)
		}
	})
	if err != nil {
		return err
	}

	// is_active(addr_ptr) -> 1|0: whether an ACTIVE contract lives at addr
	err = vm.linker.DefineAdvancedFunc("contract", "is_active", func(ins *wasman.Instance) interface{} {
		return func(addrPtr uint32) uint32 {
			vm.charge(gasKVRead)

			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			acc, err := getContract(vm.txn, addr)
			if err != nil || !acc.IsActive() {
				return 0
			}
			return 1
		}
	})
	if err != nil {
		return err
	}

	// get_code_size(addr_ptr) -> len: the byte length of the contract's
	// on-chain code at addr (0 if none)
	err = vm.linker.DefineAdvancedFunc("contract", "get_code_size", func(ins *wasman.Instance) interface{} {
		return func(addrPtr uint32) uint32 {
			vm.charge(gasKVRead)

			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			acc, err := getContract(vm.txn, addr)
			if err != nil {
				return 0
			}
			return uint32(len(acc.Source))
		}
	})
	if err != nil {
		return err
	}

	// get_code(addr_ptr, ptr) -> len: copy the contract's on-chain code at
	// addr into linear memory (size it first with get_code_size)
	err = vm.linker.DefineAdvancedFunc("contract", "get_code", func(ins *wasman.Instance) interface{} {
		return func(addrPtr, ptr uint32) uint32 {
			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			acc, err := getContract(vm.txn, addr)
			if err != nil {
				return 0
			}
			// priced per byte copied: code runs up to the source cap
			vm.charge(gasKVRead + uint64(len(acc.Source))/8)
			l, err := cp(ins, ptr, acc.Source)
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

	// code_hash(addr_ptr, ptr) -> 32|0: write the 32-byte blake3 of the
	// contract's code at addr (0 if none). Lets a caller pin the exact
	// implementation it trusts — verify a dependency, or a proxy its impl —
	// without reading the whole code back
	err = vm.linker.DefineAdvancedFunc("contract", "code_hash", func(ins *wasman.Instance) interface{} {
		return func(addrPtr, ptr uint32) uint32 {
			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			acc, err := getContract(vm.txn, addr)
			if err != nil || len(acc.Source) == 0 {
				return 0
			}
			// priced per byte hashed: blake3 over up to the source cap
			vm.charge(gasKVRead + uint64(len(acc.Source))/8)
			l, err := cp(ins, ptr, utils.Hash256(acc.Source))
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
