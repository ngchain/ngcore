package ngtypes

import (
	"bytes"
	"crypto/ecdsa"
	"errors"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/utils"
)

var (
	ErrAccountNotExists        = errors.New("the account does not exist")
	ErrAccountBalanceNotExists = errors.New("account's balance is missing")
	ErrMalformedSheet          = errors.New("the sheet structure is malformed")
)

// NewSheet gets the rows from db and return the sheet for transport/saving
func NewSheet(accounts map[uint64]*Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		Version:   Version,
		Accounts:  accounts,
		Anonymous: anonymous,
	}
}

func NewEmptySheet() *Sheet {
	return &Sheet{
		Version:   Version,
		Accounts:  map[uint64]*Account{},
		Anonymous: map[string][]byte{},
	}
}

func (m *Sheet) RegisterAccount(account *Account) error {
	if m.Accounts[account.ID] != nil {
		return errors.New("failed to register, account already exists")
	}

	m.Accounts[account.ID] = account
	return nil
}

func (m *Sheet) GetAccountByID(accountID uint64) (*Account, error) {
	if !m.HasAccount(accountID) {
		return nil, errors.New("no such account")
	}

	return m.Accounts[accountID], nil
}

func (m *Sheet) GetAccountByKey(publicKey ecdsa.PublicKey) ([]*Account, error) {
	accounts := make([]*Account, 0)
	bPublicKey := utils.ECDSAPublicKey2Bytes(publicKey)

	for i := range m.Accounts {
		if bytes.Equal(m.Accounts[i].Owner, bPublicKey) {
			accounts = append(accounts, m.Accounts[i])
		}
	}

	return accounts, nil
}

func (m *Sheet) GetAccountByKeyBytes(bPublicKey []byte) ([]*Account, error) {
	accounts := make([]*Account, 0)
	for i := range m.Accounts {
		if !bytes.Equal(m.Accounts[i].Owner, bPublicKey) {
			accounts = append(accounts, m.Accounts[i])
		}
	}

	return accounts, nil
}

func (m *Sheet) HasAccount(accountID uint64) bool {
	return m.Accounts[accountID] != nil
}

func (m *Sheet) DelAccount(accountID uint64) error {
	if !m.HasAccount(accountID) {
		return errors.New("no such account")
	}

	m.Accounts[accountID] = nil
	return nil
}

func (m *Sheet) ExportAccounts() []*Account {
	accounts := make([]*Account, len(m.Accounts))
	for i, row := range m.Accounts {
		accounts[i] = row
	}
	return accounts
}

func (m *Sheet) Copy() *Sheet {
	s := proto.Clone(m).(*Sheet)
	return s
}

func (m *Sheet) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}
