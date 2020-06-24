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
func (s *State) ToSheet() *ngtypes.Sheet {
	s.RLock()
	defer s.RUnlock()

	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	var err error
	for height, raw := range s.accounts {
		account := new(ngtypes.Account)
		err = utils.Proto.Unmarshal(raw, account)
		if err != nil {
			panic(err)
		}
		accounts[height] = account
	}

	for bs58PK, balance := range s.anonymous {
		anonymous[bs58PK] = balance
	}

	return ngtypes.NewSheet(s.height, accounts, anonymous)
}

// GetBalanceByNum get the balance of account by the account's num
func (s *State) GetBalanceByNum(id uint64) (*big.Int, error) {
	s.RLock()
	defer s.RUnlock()

	rawAccount, exists := s.accounts[id]
	if !exists {
		return nil, fmt.Errorf("account does not exists")
	}

	account := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawAccount, account)
	if err != nil {
		return nil, err
	}

	publicKey := base58.FastBase58Encoding(account.Owner)

	rawBalance, exists := s.anonymous[publicKey]
	if !exists {
		return nil, fmt.Errorf("account balance does not exists")
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

// GetBalanceByAddress get the balance of account by the account's publickey
func (s *State) GetBalanceByAddress(address ngtypes.Address) (*big.Int, error) {
	s.RLock()
	defer s.RUnlock()

	rawBalance, exists := s.anonymous[base58.FastBase58Encoding(address)]
	if !exists {
		return nil, fmt.Errorf("account balance does not exist")
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

// AccountIsRegistered checks whether the account is registered in state
func (s *State) AccountIsRegistered(num uint64) bool {
	s.RLock()
	defer s.RUnlock()

	_, exists := s.accounts[num]
	return exists
}

// GetAccountByNum returns an ngtypes.Account obj by the account's number
func (s *State) GetAccountByNum(num uint64) (account *ngtypes.Account, err error) {
	s.RLock()
	defer s.RUnlock()

	rawAccount, exists := s.accounts[num]
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

// GetAccountsByAddress returns an ngtypes.Account obj by the account's publickey
func (s *State) GetAccountsByAddress(publicKey []byte) ([]*ngtypes.Account, error) {
	s.RLock()
	defer s.RUnlock()

	accounts := make([]*ngtypes.Account, 0)
	var err error
	for _, raw := range s.accounts {
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
