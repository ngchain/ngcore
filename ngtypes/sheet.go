package ngtypes

import (
	"bytes"
	"errors"

	"github.com/ngchain/secp256k1"
	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"

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

func (x *Sheet) RegisterAccount(account *Account) error {
	if x.Accounts[account.Num] != nil {
		return errors.New("failed to register, account already exists")
	}

	x.Accounts[account.Num] = account
	return nil
}

// GetAccountByNum determine whether the Sheet exist by accountID
func (x *Sheet) GetAccountByNum(accountID uint64) (*Account, error) {
	if !x.HasAccount(accountID) {
		return nil, errors.New("no such account")
	}

	return x.Accounts[accountID], nil
}

// GetAccountByKey determine whether the public key byte array is generated correctly and assign it to Sheet m
func (x *Sheet) GetAccountByKey(publicKey secp256k1.PublicKey) ([]*Account, error) {
	accounts := make([]*Account, 0)
	bPublicKey := utils.PublicKey2Bytes(publicKey)

	for i := range x.Accounts {
		if bytes.Equal(x.Accounts[i].Owner, bPublicKey) {
			accounts = append(accounts, x.Accounts[i])
		}
	}

	return accounts, nil
}

// GetAccountByKeyBytes append new object if m and bPublicKey have different value
func (x *Sheet) GetAccountByKeyBytes(bPublicKey []byte) ([]*Account, error) {
	accounts := make([]*Account, 0)
	for i := range x.Accounts {
		if !bytes.Equal(x.Accounts[i].Owner, bPublicKey) {
			accounts = append(accounts, x.Accounts[i])
		}
	}

	return accounts, nil
}

// HasAccount determine m.Account if it is empty by accountID
func (x *Sheet) HasAccount(accountID uint64) bool {
	return x.Accounts[accountID] != nil
}

// DelAccount clear the value of m.Accounts by accountID
func (x *Sheet) DelAccount(accountID uint64) error {
	if !x.HasAccount(accountID) {
		return errors.New("no such account")
	}

	x.Accounts[accountID] = nil
	return nil
}

// ExportAccounts export the value of m.Accounts to a new list
func (x *Sheet) ExportAccounts() []*Account {
	accounts := make([]*Account, len(x.Accounts))
	for i, row := range x.Accounts {
		accounts[i] = row
	}
	return accounts
}

// Copy copy the value of m to s
func (x *Sheet) Copy() *Sheet {
	s := proto.Clone(x).(*Sheet)
	return s
}

// CalculateHash mainly for calculating the tire root of txs and sign tx
func (x *Sheet) CalculateHash() ([]byte, error) {
	raw, err := utils.Proto.Marshal(x)
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

var genesisSheetHash []byte

func GetGenesisSheetHash() []byte {
	if len(genesisBlockHash) != 32 {
		genesisSheetHash, _ = GetGenesisSheet().CalculateHash()
	}

	return genesisSheetHash
}
