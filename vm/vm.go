package vm

import (
	"github.com/dop251/goja"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
)

// JSVM is a javascript VM for exec javascript codes from account state, based on goja
type JSVM struct {
	*goja.Runtime
}

// NewJSVM is a new javascript VM
func NewJSVM() *JSVM {
	runtime := goja.New()
	return &JSVM{
		Runtime: runtime,
	}
}

// RunContract runs the state from account
func (vm *JSVM) RunContract(raw []byte) error {
	_, err := vm.RunScript("contract", string(raw))
	if err != nil {
		return err
	}

	return nil
}

// RunInit is called when assign
func (vm *JSVM) RunInit() error {
	_, err := vm.RunString(`var contract = new Contract();`)
	if err != nil {
		return err
	}

	_, err = vm.RunString(`contract.onInit();`)
	if err != nil {
		return err
	}

	return nil
}

// RunMain returns the new state after main
func (vm *JSVM) RunMain() ([]byte, error) {
	result, err := vm.RunString(`contract.main();`)
	if err != nil {
		return nil, err
	}

	return []byte(result.String()), nil
}

type account struct {
	Num   uint64 `json:"num"`
	Owner string `json:"owner"`
	Nonce uint64 `json:"nonce"`
	State string `json:"state"`
}

func convertAccount(a *ngtypes.Account) *account {
	return &account{
		Num:   a.Num,
		Owner: base58.FastBase58Encoding(a.Owner),
		Nonce: a.Nonce,
		State: string(a.State),
	}
}

// InheritOwnerAccount transfer the Owner Account into the vm
func (vm *JSVM) InheritOwnerAccount(account *ngtypes.Account) {
	vm.Set("owner", convertAccount(account))
}
