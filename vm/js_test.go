package vm

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewJsVM(t *testing.T) {
	_ = NewJSVM()
}

func TestJsVM_RunState(t *testing.T) {
	vm := NewJSVM()
	f, _ := os.Open("./contracts/add.js")
	raw, _ := ioutil.ReadAll(f)
	if string(vm.RunState(raw)) != "3" {
		t.Fail()
	}
}
