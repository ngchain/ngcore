package ngtypes

import (
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"

	"golang.org/x/crypto/sha3"
)

var (
	ErrNotCheckpoint    = errors.New("not proper time for building new vault")
	ErrInvalidHookBlock = errors.New("the vault's hook_block is invalid")
	ErrMalformedVault   = errors.New("the vault structure is malformed")
)

func NewVault(prevVaultHeight uint64, prevVaultHash []byte, currentSheet *Sheet) *Vault {
	return &Vault{
		NetworkId:     NetworkID,
		Height:        prevVaultHeight + 1,
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

		NetworkId: NetworkID,
		Timestamp: genesisTimestamp,

		PrevVaultHash: nil,

		Sheet: &Sheet{
			Version: Version,
			Accounts: map[uint64]*Account{
				0: GetGenesisAccount(),
			},
			Anonymous: map[string][]byte{
				GenesisPKHex: GetBig0Bytes(),
			},
		},
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
