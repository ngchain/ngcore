package wasm

import (
	"reflect"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/ngstate"
)

func (vm *VM) InitBuiltInImports() error {
	err := vm.linker.Define("ng", "get_num", wasmtime.WrapFunc(vm.store, func() int64 {
		return int64(vm.num)
	}).AsExtern())
	if err != nil {
		return err
	}

	err = vm.linker.Define("ng", "debug", wasmtime.WrapFunc(vm.store, func(ptr int32, l int32) {
		message := *(*string)(unsafe.Pointer(&reflect.StringHeader{
			Data: uintptr(ptr),
			Len:  int(l),
		}))
		log.Debug(message)
	}).AsExtern())
	if err != nil {
		return err
	}

	err = vm.linker.Define("ng", "transfer", wasmtime.WrapFunc(
		vm.store, func(to, value int64) int32 {
			err := ngstate.VMTransfer(vm.num, uint64(to), uint64(value))
			if err != nil {
				log.Error(err)
				return -1
			}

			return 1
		}).AsExtern(),
	)
	if err != nil {
		return err
	}

	return nil
}
