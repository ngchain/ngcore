package ngtypes

import (
	"bytes"
)

// Account is the contract slot of an address: it exists only after the
// address paid the one-time deploy fee. The address itself is the
// namespace — no numbers, no names: every address owns exactly its own
// slot
type Contract struct {
	Owner   Address
	Source  []byte
	Context *ContractContext
}

// NewContract opens the contract slot of the owner address
func NewContract(owner Address, source []byte, context *ContractContext) *Contract {
	if context == nil {
		context = NewContractContext()
	}

	return &Contract{
		Owner:   owner,
		Source:  source,
		Context: context,
	}
}

// ContextKeyActive is the reserved context key marking the contract as
// active. Keys with the "_" prefix are reserved for the system and
// cannot be touched by contracts through the kv host module.
const ContextKeyActive = "_active"

// IsActive shows whether the contract is active: runnable by the vm,
// with the Contract field immutable
func (x *Contract) IsActive() bool {
	if x.Context == nil {
		return false
	}

	return len(x.Context.Get(ContextKeyActive)) != 0
}

// SetActive updates the active flag of the contract
func (x *Contract) SetActive(active bool) {
	if x.Context == nil {
		x.Context = NewContractContext()
	}

	if active {
		x.Context.Set(ContextKeyActive, []byte{1})
	} else {
		x.Context.Del(ContextKeyActive)
	}
}

// Equals returns whether the other is equals to the Account
func (x *Contract) Equals(other *Contract) (bool, error) {
	if x.Owner != other.Owner {
		return false, nil
	}
	if !(bytes.Equal(x.Source, other.Source)) {
		return false, nil
	}
	if eq, _ := x.Context.Equals(other.Context); !eq {
		return false, nil
	}

	return true, nil
}
