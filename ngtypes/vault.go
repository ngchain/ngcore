package ngtypes

import (
	"errors"
	"github.com/gogo/protobuf/proto"
	"time"

	"golang.org/x/crypto/sha3"
)

var (
	ErrNotCheckpoint    = errors.New("not proper time for building new vault")
	ErrInvalidHookBlock = errors.New("the vault's hook_block is invalid")
	ErrMalformedVault   = errors.New("the vault structure is malformed")
)

func NewVault(newAccountID uint64, ownerKey []byte, prevVaultHeight uint64, prevVaultHash []byte, currentSheet *Sheet) *Vault {
	newAccount := NewAccount(newAccountID, ownerKey, nil)

	return &Vault{
		NetworkId:     NetworkId,
		Height:        prevVaultHeight + 1,
		List:          newAccount,
		Timestamp:     time.Now().Unix(),
		PrevVaultHash: prevVaultHash,
		Sheet:         currentSheet,
	}
}

func (m *Vault) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}

func GetGenesisVault() *Vault {
	v := &Vault{
		Height: 0,

		NetworkId: NetworkId,
		Timestamp: genesisTimestamp,

		PrevVaultHash: nil,

		Sheet: &Sheet{
			Version:   Version,
			Accounts:  map[uint64]*Account{},
			Anonymous: map[string][]byte{},
		},

		List:    GetGenesisAccount(),
		Delists: []*Account{},
	}

	return v
}

func (m *Vault) Copy() *Vault {
	v := proto.Clone(m).(*Vault)
	return v
}

func (m *Vault) GetPrevHash() []byte {
	return m.PrevVaultHash
}

var GenesisVaultHash, _ = GetGenesisVault().CalculateHash()
