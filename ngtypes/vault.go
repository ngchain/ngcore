package ngtypes

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/sha3"
)

var (
	ErrNotCheckpoint    = errors.New("not proper time for building new vault")
	ErrInvalidHookBlock = errors.New("the vault's hook_block is invalid")
	ErrMalformedVault   = errors.New("the vault structure is malformed")
)

// NewVault default class constructor
func NewVault(prevVaultHeight uint64, prevVaultHash []byte, currentSheet *Sheet) *Vault {
	return &Vault{
		NetworkId:     NetworkID,
		Height:        prevVaultHeight + 1,
		Timestamp:     time.Now().Unix(),
		PrevVaultHash: prevVaultHash,
		Sheet:         currentSheet,
	}
}

// CalculateHash mainly for calculating the tire root of txs and sign tx
func (m *Vault) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}

// GetGenesisVault return Value v
func GetGenesisVault() *Vault {
	v := &Vault{
		Height: 0,

		NetworkId: NetworkID,
		Timestamp: genesisTimestamp,

		PrevVaultHash: nil,

		Sheet: GetGenesisSheet(),
	}

	return v
}

func (m *Vault) CheckError() error {
	if m.NetworkId != NetworkID {
		return fmt.Errorf("vault's network id is incorrect")
	}
	return nil
}

func (m *Vault) GetPrevHash() []byte {
	return m.PrevVaultHash
}

var GenesisVaultHash, _ = GetGenesisVault().CalculateHash()
