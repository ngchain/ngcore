package ngtypes

import (
	"bytes"
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/ngchain/secp256k1"
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
		Accounts:  accounts,
		Anonymous: anonymous,
	}
}

func (m *Sheet) RegisterAccount(account *Account) error {
	if m.Accounts[account.Num] != nil {
		return errors.New("failed to register, account already exists")
	}

	m.Accounts[account.Num] = account
	return nil
}

// GetAccountByID determine whether the Sheet exist by accountID
func (m *Sheet) GetAccountByID(accountID uint64) (*Account, error) {
	if !m.HasAccount(accountID) {
		return nil, errors.New("no such account")
	}

	return m.Accounts[accountID], nil
}

// GetAccountByKey determine whether the public key byte array is generated correctly and assign it to Sheet m
func (m *Sheet) GetAccountByKey(publicKey secp256k1.PublicKey) ([]*Account, error) {
	accounts := make([]*Account, 0)
	bPublicKey := utils.PublicKey2Bytes(publicKey)

	for i := range m.Accounts {
		if bytes.Equal(m.Accounts[i].Owner, bPublicKey) {
			accounts = append(accounts, m.Accounts[i])
		}
	}

	return accounts, nil
}

// GetAccountByKeyBytes append new object if m and bPublicKey have different value
func (m *Sheet) GetAccountByKeyBytes(bPublicKey []byte) ([]*Account, error) {
	accounts := make([]*Account, 0)
	for i := range m.Accounts {
		if !bytes.Equal(m.Accounts[i].Owner, bPublicKey) {
			accounts = append(accounts, m.Accounts[i])
		}
	}

	return accounts, nil
}

// HasAccount determine m.Account if it is empty by accountID
func (m *Sheet) HasAccount(accountID uint64) bool {
	return m.Accounts[accountID] != nil
}

// DelAccount clear the value of m.Accounts by accountID
func (m *Sheet) DelAccount(accountID uint64) error {
	if !m.HasAccount(accountID) {
		return errors.New("no such account")
	}

	m.Accounts[accountID] = nil
	return nil
}

// ExportAccounts export the value of m.Accounts to a new list
func (m *Sheet) ExportAccounts() []*Account {
	accounts := make([]*Account, len(m.Accounts))
	for i, row := range m.Accounts {
		accounts[i] = row
	}
	return accounts
}

// Copy copy the value of m to s
func (m *Sheet) Copy() *Sheet {
	s := proto.Clone(m).(*Sheet)
	return s
}

// CalculateHash mainly for calculating the tire root of txs and sign tx
func (m *Sheet) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}

func GetGenesisSheet() *Sheet {
	// reserve 1-10 to provide official functions
	genesisAccounts := make(map[uint64]*Account)
	for i := uint64(0); i <= 10; i++ {
		genesisAccounts[i] = GetGenesisAccount(i)
	}

	return &Sheet{
		Accounts: genesisAccounts,
		Anonymous: map[string][]byte{
			GenesisPublicKeyBase58: GetBig0Bytes(),
		},
	}
}

var GenesisSheetHash, _ = GetGenesisSheet().CalculateHash()
