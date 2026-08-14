package ngtypes

import (
	"bytes"
)

// Account is the contract slot of an address: it exists only after the
// address paid the one-time deploy fee. The address itself is the
// namespace — no numbers, no names: every address owns exactly its own
// slot
type Account struct {
	Owner    Address
	Contract []byte
	Context  *AccountContext
}

// NewAccount opens the contract slot of the owner address
func NewAccount(owner Address, contract []byte, context *AccountContext) *Account {
	if context == nil {
		context = NewAccountContext()
	}

	return &Account{
		Owner:    owner,
		Contract: contract,
		Context:  context,
	}
}

// ContextKeyLock is the reserved context key marking the account as locked.
// Keys with the "_" prefix are reserved for the system and cannot be
// touched by contracts through the kv host module.
const ContextKeyLock = "_locked"

// IsLocked shows whether the account is locked: its contract is active
// (runnable by the vm) and its Contract field is immutable
func (x *Account) IsLocked() bool {
	if x.Context == nil {
		return false
	}

	return len(x.Context.Get(ContextKeyLock)) != 0
}

// SetLock updates the lock flag of the account
func (x *Account) SetLock(locked bool) {
	if x.Context == nil {
		x.Context = NewAccountContext()
	}

	if locked {
		x.Context.Set(ContextKeyLock, []byte{1})
	} else {
		x.Context.Del(ContextKeyLock)
	}
}

// Equals returns whether the other is equals to the Account
func (x *Account) Equals(other *Account) (bool, error) {
	if x.Owner != other.Owner {
		return false, nil
	}
	if !(bytes.Equal(x.Contract, other.Contract)) {
		return false, nil
	}
	if eq, _ := x.Context.Equals(other.Context); !eq {
		return false, nil
	}

	return true, nil
}
