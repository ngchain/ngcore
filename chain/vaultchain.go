package chain

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"go.etcd.io/bbolt"
)

type VaultChain struct {
	DB *StorageChain
}

func NewVaultChain(db *bbolt.DB) *VaultChain {
	sc := NewStorageChain([]byte("vault"), db, ngtypes.GetGenesisVault())
	return &VaultChain{
		DB: sc,
	}
}

// TODO
// VerifyChain is a healthy check for VaultChain
func (vc *VaultChain) VerifyChain() error {
	return nil
}

// GetLatestVault returns the latest vault in DB
func (vc *VaultChain) GetLatestVault() *ngtypes.Vault {
	item, err := vc.DB.GetLatestItem(new(ngtypes.Vault))
	if err == nil && item != nil {
		return item.(*ngtypes.Vault)
	}

	log.Error(err)
	return nil
}

// GetLatestVaultHash returns the latest vault's hash in DB
func (vc *VaultChain) GetLatestVaultHash() []byte {
	hash, err := vc.DB.GetLatestHash()
	if err == nil && hash != nil {
		return hash
	}

	log.Error(err)

	return nil
}

// GetLatestVaultHeight returns the latest vault's height in DB
func (vc *VaultChain) GetLatestVaultHeight() uint64 {
	height, err := vc.DB.GetLatestHeight()
	if err == nil && height != 0 {
		return height
	}

	log.Error(err)

	return 0
}

// PutVault puts an vault into DB
func (vc *VaultChain) PutVault(vault *ngtypes.Vault) error {
	err := vc.DB.PutItem(vault)
	if err != nil {
		return err
	}

	return nil
}
