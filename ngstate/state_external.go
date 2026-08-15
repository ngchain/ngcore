package ngstate

import (
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// GetTotalBalanceByAddress get the total balance of account by the account's address
func (state *State) GetTotalBalanceByAddress(address ngtypes.Address) (*big.Int, error) {
	var balance *big.Int

	err := state.View(func(txn *bbolt.Tx) error {
		balance = getBalance(txn, address)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// GetMatureBalanceByAddress get the locked balance of account by the account's address
func (state *State) GetMatureBalanceByAddress(address ngtypes.Address) (*big.Int, error) {
	balance := big.NewInt(0)

	err := state.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		var err error

		currentHeight, err := ngblocks.GetLatestHeight(blockBucket)
		if err != nil {
			return err
		}

		matureSnapshot := state.GetSnapshotByHeight(ngtypes.GetMatureHeight(currentHeight))
		if matureSnapshot == nil {
			return errors.Wrap(ErrSnapshotNofFound, "cannot find the mature snapshot") // abnormal
		}

		for i := range matureSnapshot.Balances {
			if matureSnapshot.Balances[i].Address == address {
				balance = matureSnapshot.Balances[i].Amount
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// GetContract returns the contract slot of the address, if it
// ever deployed
func (state *State) GetContract(address ngtypes.Address) (*ngtypes.Contract, error) {
	var account *ngtypes.Contract
	err := state.View(func(txn *bbolt.Tx) error {
		var err error
		account, err = getContract(txn, address)
		return err
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}

// PubKeyRegistered reports whether the address's public key is already
// on chain, so wallets can switch to the compact envelope
func (state *State) PubKeyRegistered(addr ngtypes.Address) bool {
	registered := false
	_ = state.View(func(txn *bbolt.Tx) error {
		registered = txn.Bucket(storage.KeyRegistryBucketName).Get(addr[:]) != nil
		return nil
	})

	return registered
}
