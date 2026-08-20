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

// ErrArchiveDisabled is returned by the *AtHeight readers on a node that
// did not retain historical state
var ErrArchiveDisabled = errors.New("archive is not enabled on this node")

// GetBalanceByAddressAt resolves an address's balance as of the given
// block height, from the changeset history. Requires an archive node
func (state *State) GetBalanceByAddressAt(address ngtypes.Address, height uint64) (*big.Int, error) {
	if !state.Archive {
		return nil, ErrArchiveDisabled
	}

	var balance *big.Int
	err := state.View(func(txn *bbolt.Tx) error {
		if err := checkHeightInRange(txn, height); err != nil {
			return err
		}
		balance = balanceAtHeight(txn, address, height)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// GetContractAt resolves an address's contract slot as of the given block
// height, from the changeset history. Requires an archive node;
// ErrKeyNotFound when the address had no slot at that height
func (state *State) GetContractAt(address ngtypes.Address, height uint64) (*ngtypes.Contract, error) {
	if !state.Archive {
		return nil, ErrArchiveDisabled
	}

	var account *ngtypes.Contract
	err := state.View(func(txn *bbolt.Tx) error {
		if err := checkHeightInRange(txn, height); err != nil {
			return err
		}
		acc, ok, err := contractAtHeight(txn, address, height)
		if err != nil {
			return err
		}
		if !ok {
			return errors.Wrapf(storage.ErrKeyNotFound, "no contract slot on %s at height %d", address, height)
		}
		account = acc
		return nil
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}

// checkHeightInRange rejects a query the archive cannot answer truthfully.
// Above the chain tip has no defined state. Below the tip, the resolver
// relies on changesets covering (height, tip]: a missing changeset at
// height+1 means no recorded history reaches this height — a
// snapshot-started node below its checkpoint, or a pre-archive db not yet
// backfilled — so it is refused rather than answered with current state.
// (Every applied archive height carries the coinbase balance change, so
// the presence of height+1 implies the whole range above it is covered.)
func checkHeightInRange(txn *bbolt.Tx, height uint64) error {
	tip, err := ngblocks.GetLatestHeight(txn.Bucket(storage.BlockBucketName))
	if err != nil {
		return err
	}
	if height > tip {
		return errors.Errorf("height %d is above the chain tip %d", height, tip)
	}
	// at the tip the current plain state IS the answer; below it, require
	// recorded coverage
	if height < tip && !changesetCovers(txn, height+1) {
		return errors.Errorf("no archived history at height %d (below the retained range)", height)
	}

	return nil
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
