package vm_test

import (
	"fmt"
	"testing"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/hive/vm"
)

func TestNewWasmVM(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(import "" "hello" (func $hello))
		(func (export "_start")
			(call $hello)
		)
		(func (export "size") (result i32) (memory.size))
        (func (export "load") (param i32) (result i32)
			(i32.load8_s (local.get 0))
		)
		(func (export "store") (param i32 i32)
			(i32.store8 (local.get 0) (local.get 1))
		)
		(memory (export "memory") 2 3)
		(data (i32.const 0x1000) "\01\02\03\04")
	)`)
	if err != nil {
		panic(err)
	}

	contract, err := vm.NewWasmVM(wasm)
	if err != nil {
		panic(err)
	}

	item := wasmtime.WrapFunc(contract.GetStore(), func() {
		fmt.Println("Hello from Go!")
	})
	err = contract.Instantiate(item.AsExtern())
	if err != nil {
		panic(err)
	}

	contract.Start()
}
