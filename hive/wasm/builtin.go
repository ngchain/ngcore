package wasm

import (
	"reflect"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/ngstate"
)

func (vm *VM) InitBuiltInImports() error {
	if err := initLogImports(vm); err != nil {
		return err
	}

	if err := initSelfImports(vm); err != nil {
		return err
	}

	if err := initCoinImports(vm); err != nil {
		return err
	}

	return nil
}

func initLogImports(vm *VM) error {
	err := vm.linker.Define("log", "debug", wasmtime.WrapFunc(vm.store, func(ptr int32, l int32) {
		message := *(*string)(unsafe.Pointer(&reflect.StringHeader{
			Data: uintptr(ptr),
			Len:  int(l),
		}))
		vm.logger.Debug(message)
	}).AsExtern())
	if err != nil {
		return err
	}

	return nil
}

func initSelfImports(vm *VM) error {
	err := vm.linker.Define("self", "get_num", wasmtime.WrapFunc(vm.store, func() int64 {
		return int64(vm.self.Num)
	}).AsExtern())
	if err != nil {
		return err
	}

	err = vm.linker.Define("self", "get_owner", wasmtime.WrapFunc(vm.store, func() int64 {
		return int64(vm.self.Num)
	}).AsExtern())
	if err != nil {
		return err
	}

	return nil
}

func initCoinImports(vm *VM) error {
	err := vm.linker.Define("coin", "transfer", wasmtime.WrapFunc(
		vm.store, func(to, value int64) int32 {
			err := ngstate.VMTransfer(vm.self.Num, uint64(to), uint64(value))
			if err != nil {
				vm.logger.Error(err)
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
