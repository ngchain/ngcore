package vm

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

func TestNewJsVM(t *testing.T) {
	_ = NewJSVM()
}

func TestJsVM_RunMain(t *testing.T) {
	vm := NewJSVM()
	f, _ := os.Open("./contracts/add.js")
	raw, _ := ioutil.ReadAll(f)
	err := vm.RunContract(raw)
	if err != nil {
		t.Log(err)
	}
	err = vm.RunInit()
	if err != nil {
		t.Log(err)
	}
	result, err := vm.RunMain()
	if err != nil {
		t.Log(err)
	}
	mainResult := string(result)
	if mainResult != "3" {
		t.Fail()
	}
}

func TestJSVM_InheritOwnerAccount(t *testing.T) {
	vm := NewJSVM()
	f, _ := os.Open("./contracts/concurrency.js")
	raw, _ := ioutil.ReadAll(f)
	account := ngtypes.GetGenesisAccount(10)

	err := vm.RunContract(raw)
	if err != nil {
		t.Log(err)
	}
	vm.InheritOwnerAccount(account)
	err = vm.RunInit()
	if err != nil {
		t.Log(err)
	}

	result, err := vm.RunMain()
	if err != nil {
		t.Log(err)
	}
	mainResult := string(result)

	if mainResult != "ngchain" {
		t.Log(mainResult)
		t.Fail()
	}
}
