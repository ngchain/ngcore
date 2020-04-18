package vm

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestWasmVM_RunMain(t *testing.T) {
	f, _ := os.Open("./contracts/rust/sum.wasm")
	raw, _ := ioutil.ReadAll(f)
	vm, err := NewWasmVM(raw)
	if err != nil {
		t.Log(err)
	}
	result, err := vm.Exports["sum"](0, 3)
	if err != nil {
		t.Log(err)
	}
	if result.ToI64() != 3 {
		t.Fail()
	}
}
