package ngsheet

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type state struct {
	sync.RWMutex

	// using bytes to keep data safe
	accounts  map[uint64][]byte
	anonymous map[string][]byte
}

// newStateFromSheet will create a new state which is a wrapper of *ngtypes.sheet
func newStateFromSheet(sheet *ngtypes.Sheet) (*state, error) {
	entry := &state{
		accounts:  make(map[uint64][]byte),
		anonymous: make(map[string][]byte),
	}

	var err error
	for id, account := range sheet.Accounts {
		entry.accounts[id], err = utils.Proto.Marshal(account)
		if err != nil {
			return nil, err
		}
	}

	for bs58PK, balance := range sheet.Anonymous {
		entry.anonymous[bs58PK] = balance
	}

	return entry, nil
}

// ToSheet will conclude a sheet which has all status of all accounts & keys(if balance not nil)
func (m *state) ToSheet() (*ngtypes.Sheet, error) {
	m.RLock()
	defer m.RUnlock()

	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	var err error
	for height, raw := range m.accounts {
		account := new(ngtypes.Account)
		err = utils.Proto.Unmarshal(raw, account)
		if err != nil {
			return nil, err
		}
		accounts[height] = account
	}

	for bs58PK, balance := range m.anonymous {
		anonymous[bs58PK] = balance
	}

	return ngtypes.NewSheet(accounts, anonymous), nil
}

func (m *state) GetBalanceByNum(id uint64) (*big.Int, error) {
	m.RLock()
	defer m.RUnlock()

	rawAccount, exists := m.accounts[id]
	if !exists {
		return nil, fmt.Errorf("account does not exists")
	}

	account := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawAccount, account)
	if err != nil {
		return nil, err
	}

	publicKey := base58.FastBase58Encoding(account.Owner)

	rawBalance, exists := m.anonymous[publicKey]
	if !exists {
		return nil, fmt.Errorf("account balance does not exists")
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

func (m *state) getBalanceByPublicKey(publicKey []byte) (*big.Int, error) {
	m.RLock()
	defer m.RUnlock()

	rawBalance, exists := m.anonymous[base58.FastBase58Encoding(publicKey)]
	if !exists {
		return nil, fmt.Errorf("account balance does not exist")
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

func (m *state) accountIsRegistered(num uint64) bool {
	m.RLock()
	defer m.RUnlock()

	_, exists := m.accounts[num]
	return exists
}

func (m *state) getAccountByNum(num uint64) (account *ngtypes.Account, err error) {
	m.RLock()
	defer m.RUnlock()

	rawAccount, exists := m.accounts[num]
	if !exists {
		return nil, fmt.Errorf("account does not exist")
	}

	account = new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawAccount, account)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (m *state) getAccountsByPublicKey(publicKey []byte) ([]*ngtypes.Account, error) {
	m.RLock()
	defer m.RUnlock()

	accounts := make([]*ngtypes.Account, 0)
	var err error
	for _, raw := range m.accounts {
		account := new(ngtypes.Account)
		err = utils.Proto.Unmarshal(raw, account)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(account.Owner, publicKey) {
			accounts = append(accounts, account)
		}
	}

	return accounts, nil
}
