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

func NewVault(newAccountID uint64, prevVault *Vault, hookBlock *Block, currentSheet *Sheet) *Vault {
	if !hookBlock.Header.IsCheckpoint() {
		log.Error(ErrNotCheckpoint)
		return nil
	}

	if !hookBlock.Header.IsSealed() {
		log.Error(ErrBlockIsUnsealing)
		return nil
	}

	if !hookBlock.VerifyNonce() {
		log.Error(ErrInvalidHookBlock)
	}

	prevVaultHash, err := prevVault.CalculateHash()
	if err != nil {
		log.Error(err)
	}

	newAccount := NewAccount(newAccountID, hookBlock.Transactions[0].Header.Participants[0], nil)

	blockHash := hookBlock.Header.CalculateHash()

	return &Vault{
		Height:        hookBlock.Header.Height / CheckRound,
		List:          newAccount,
		Timestamp:     time.Now().Unix(),
		PrevVaultHash: prevVaultHash,
		HookBlockHash: blockHash,
		Sheet:         currentSheet,
	}
}

func (m *Vault) CalculateHash() ([]byte, error) {
	v := m.Copy()
	raw, err := proto.Marshal(v)
	hash := sha3.Sum256(raw)
	return hash[:], err
}

func GetGenesisVault() *Vault {
	var hookGenesisBlock = GetGenesisBlock()
	blockHash := hookGenesisBlock.Header.CalculateHash()

	v := &Vault{
		Height: 0,

		NetworkId: NetworkId,
		Timestamp: genesisTimestamp,

		PrevVaultHash: nil,
		HookBlockHash: blockHash,

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
