package vm

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewLuaVM(t *testing.T) {
	_ = NewLuaVM()
}

func TestLuaVM_RunState(t *testing.T) {
	vm := NewLuaVM()
	f, _ := os.Open("./contracts/add.lua")
	raw, _ := ioutil.ReadAll(f)
	if string(vm.RunState(raw)) != "3" {
		t.Fail()
	}

	vm.Close()
}
