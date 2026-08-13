package ngtypes

import (
	"bytes"
)

// Account is the shell of the address to process the txs and contracts
type Account struct {
	Num      uint64
	Owner    Address
	Contract []byte
	Context  *AccountContext
}

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num AccountNum, ownerAddress Address, contract []byte, context *AccountContext) *Account {
	if context == nil {
		context = NewAccountContext()
	}

	return &Account{
		Num:      uint64(num),
		Owner:    ownerAddress,
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

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num AccountNum) *Account {
	return NewAccount(num, GenesisAddress, nil, nil)
}

// Equals returns whether the other is equals to the Account
func (x *Account) Equals(other *Account) (bool, error) {
	if !(x.Num == other.Num) {
		return false, nil
	}
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
