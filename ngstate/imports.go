package ngstate

import (
	"github.com/c0mm4nd/wasman"
)

// initBuiltInImports binds all host modules to the contract linker.
//
// The full host ABI (all funcs use the wasm32 basic types only):
//
//	log:     debug(ptr, size)  error(ptr, size)
//	account: get_host() u64
//	         get_owner_size() u32           get_owner(num u64, ptr) u32
//	         get_contract_size(num u64) u32 get_contract(num u64, ptr) u32
//	         is_locked(num u64) u32
//	coin:    get_balance_size(num u64) u32  get_balance(num u64, ptr) u32
//	         transfer(to u64, value u64) u32
//	kv:      get_size(kptr, klen) u32       get(kptr, klen, vptr) u32
//	         set(kptr, klen, vptr, vlen) u32  del(kptr, klen) u32
//	tx:      get_hash_size() u32   get_hash(ptr) u32
//	         get_network() u32     get_height() u64    get_convener() u64
//	         get_participants_count() u32
//	         get_participant_size() u32     get_participant(i, ptr) u32
//	         get_value_size(i) u32          get_value(i, ptr) u32
//	         get_fee_size() u32             get_fee(ptr) u32
//	         get_extra_size() u32           get_extra(ptr) u32
func (vm *VM) initBuiltInImports() error {
	for _, init := range []func(*VM) error{
		initLogImports,
		initAccountImports,
		initCoinImports,
		initKVImports,
		initTxImports,
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

	return nil
}
