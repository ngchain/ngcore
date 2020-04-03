package ngsheet

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

type sheetEntry struct {
	sync.RWMutex

	// using bytes to keep data safe
	accounts  map[uint64][]byte
	anonymous map[string][]byte
}

func NewSheetEntry(sheet *ngtypes.Sheet) (*sheetEntry, error) {
	entry := &sheetEntry{
		accounts:  make(map[uint64][]byte),
		anonymous: make(map[string][]byte),
	}

	var err error
	for id, account := range sheet.Accounts {
		entry.accounts[id], err = account.Marshal()
		if err != nil {
			return nil, err
		}
	}

	for hexPK, balance := range sheet.Anonymous {
		entry.anonymous[hexPK] = balance
	}

	return entry, nil
}

// ToSheet will conclude a sheet which has all status of all accounts & keys(if balance not nil)
func (m *sheetEntry) ToSheet() (*ngtypes.Sheet, error) {
	m.RLock()
	defer m.RUnlock()

	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	var err error
	for height, raw := range m.accounts {
		account := new(ngtypes.Account)
		err = account.Unmarshal(raw)
		if err != nil {
			return nil, err
		}
		accounts[height] = account
	}

	for hexPK, balance := range m.anonymous {
		anonymous[hexPK] = balance
	}

	return ngtypes.NewSheet(accounts, anonymous), nil
}

func (m *sheetEntry) GetBalanceByID(id uint64) (*big.Int, error) {
	m.RLock()
	defer m.RUnlock()

	rawAccount, exists := m.accounts[id]
	if !exists {
		return nil, ngtypes.ErrAccountNotExists
	}

	account := new(ngtypes.Account)
	err := account.Unmarshal(rawAccount)
	if err != nil {
		return nil, err
	}

	publicKey := hex.EncodeToString(account.Owner)

	rawBalance, exists := m.anonymous[publicKey]
	if !exists {
		return nil, ngtypes.ErrAccountBalanceNotExists
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

func (m *sheetEntry) GetBalanceByPublicKey(publicKey []byte) (*big.Int, error) {
	m.RLock()
	defer m.RUnlock()

	rawBalance, exists := m.anonymous[hex.EncodeToString(publicKey)]
	if !exists {
		return nil, ngtypes.ErrAccountBalanceNotExists
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

func (m *sheetEntry) AccountIsRegistered(accountID uint64) bool {
	m.RLock()
	defer m.RUnlock()

	_, exists := m.accounts[accountID]
	return exists
}

func (m *sheetEntry) GetAccountByID(id uint64) (account *ngtypes.Account, err error) {
	m.RLock()
	defer m.RUnlock()

	rawAccount, exists := m.accounts[id]
	if !exists {
		return nil, ngtypes.ErrAccountNotExists
	}

	err = account.Unmarshal(rawAccount)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (m *sheetEntry) GetAccountsByPublicKey(publicKey []byte) ([]*ngtypes.Account, error) {
	m.RLock()
	defer m.RUnlock()

	accounts := make([]*ngtypes.Account, 0)
	var err error
	for _, raw := range m.accounts {
		account := new(ngtypes.Account)
		err = account.Unmarshal(raw)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(account.Owner, publicKey) {
			accounts = append(accounts, account)
		}
	}

	return accounts, nil
}

// RegisterAccounts is same to balanceSheet RegisterAccount, this is for consensus calling
func (m *sheetEntry) RegisterAccounts(accounts ...*ngtypes.Account) (ok bool) {
	m.Lock()
	defer m.Unlock()

	ok = false

	newAccounts := make(map[uint64][]byte)
	for i := range m.accounts {
		newAccounts[i] = make([]byte, len(m.accounts[i]))
		copy(newAccounts[i], m.accounts[i])
	}

	// newAnonymous := make(map[string][]byte)
	// for i := range m.anonymous {
	// 	newAnonymous[i] = make([]byte, len(m.anonymous[i]))
	// 	copy(newAnonymous[i], m.anonymous[i])
	// }

	defer func() {
		if ok {
			m.accounts = newAccounts
			// m.anonymous = newAnonymous
		}
	}()

	var err error
	for i := range accounts {
		if _, exists := newAccounts[accounts[i].ID]; exists {
			log.Infof("failed to register account@%d", accounts[i].ID)
			return ok
		}

		newAccounts[accounts[i].ID], err = accounts[i].Marshal()
		if err != nil {
			log.Error(err)
			return ok
		}
		log.Infof("registered new account@%d", accounts[i].ID)
	}

	ok = true
	return ok
}

func (m *sheetEntry) DeleteAccounts(accounts ...*ngtypes.Account) (ok bool) {
	m.Lock()
	defer m.Unlock()

	newAccounts := make(map[uint64][]byte)
	for i := range m.accounts {
		newAccounts[i] = make([]byte, len(m.accounts[i]))
		copy(newAccounts[i], m.accounts[i])
	}

	ok = false
	defer func() {
		if ok {
			m.accounts = newAccounts
			// m.anonymous = newAnonymous
		}
	}()
	for i := range accounts {
		if _, exists := newAccounts[accounts[i].ID]; !exists {
			log.Infof("failed to delete account@%d", accounts[i].ID)
			return false
		}

		delete(newAccounts, accounts[i].ID)
		log.Infof("deleted account@%d", accounts[i].ID)
	}

	ok = true
	return true
}
