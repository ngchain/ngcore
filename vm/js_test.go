package vm

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewJsVM(t *testing.T) {
	_ = NewJsVM()
}

func TestJsVM_RunState(t *testing.T) {
	vm := NewJsVM()
	f, _ := os.Open("./contracts/add.js")
	raw, _ := ioutil.ReadAll(f)
	if string(vm.RunState(raw)) != "3" {
		t.Fail()
	}
}
