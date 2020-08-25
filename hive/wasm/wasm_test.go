package wasm_test

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/hive/wasm"
)

func TestNewWasmVM(t *testing.T) {
	raw, err := wasmtime.Wat2Wasm(``) // TODO: implement a mvp
	if err != nil {
		panic(err)
	}

	contract, err := wasm.NewVM(raw)
	if err != nil {
		panic(err)
	}

	err = contract.InitBuiltInImports()
	if err != nil {
		panic(err)
	}

	err = contract.Instantiate()
	if err != nil {
		panic(err)
	}

	contract.Start()
}
