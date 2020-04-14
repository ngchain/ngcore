package vm

import (
	"github.com/ngchain/ngcore/ngtypes"
)

// Manager is a manager to control the life circle of state vms
// TODO
type Manager struct {
	vms map[uint64]*JSVM
}

func NewVMManager() *Manager {
	return &Manager{
		vms: map[uint64]*JSVM{},
	}
}

func (m *Manager) CreateVM(account *ngtypes.Account) *JSVM {
	vm := NewJSVM()

	vm.RunContract(account.Contract)
	m.vms[account.Num] = vm

	return vm
}

func (m *Manager) CalledByTx(tx *ngtypes.Tx) {

}
