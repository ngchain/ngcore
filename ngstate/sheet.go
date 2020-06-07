package ngstate

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ToSheet will conclude a sheet which has all status of all accounts & keys(if balance not nil)
func (m *State) ToSheet() *ngtypes.Sheet {
	m.RLock()
	defer m.RUnlock()

	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	var err error
	for height, raw := range m.accounts {
		account := new(ngtypes.Account)
		err = utils.Proto.Unmarshal(raw, account)
		if err != nil {
			panic(err)
		}
		accounts[height] = account
	}

	for bs58PK, balance := range m.anonymous {
		anonymous[bs58PK] = balance
	}

	return ngtypes.NewSheet(m.height, accounts, anonymous)
}

// GetBalanceByNum get the balance of account by the account's num
func (m *State) GetBalanceByNum(id uint64) (*big.Int, error) {
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

// GetBalanceByPublicKey get the balance of account by the account's publickey
func (m *State) GetBalanceByPublicKey(publicKey []byte) (*big.Int, error) {
	m.RLock()
	defer m.RUnlock()

	rawBalance, exists := m.anonymous[base58.FastBase58Encoding(publicKey)]
	if !exists {
		return nil, fmt.Errorf("account balance does not exist")
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

// AccountIsRegistered checks whether the account is registered in state
func (m *State) AccountIsRegistered(num uint64) bool {
	m.RLock()
	defer m.RUnlock()

	_, exists := m.accounts[num]
	return exists
}

// GetAccountByNum returns an ngtypes.Account obj by the account's number
func (m *State) GetAccountByNum(num uint64) (account *ngtypes.Account, err error) {
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

// GetAccountsByPublicKey returns an ngtypes.Account obj by the account's publickey
func (m *State) GetAccountsByPublicKey(publicKey []byte) ([]*ngtypes.Account, error) {
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
