package ngstate

import (
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// getContract loads the contract slot of an address; ErrKeyNotFound
// when the address never deployed
func getContract(txn *bbolt.Tx, addr ngtypes.Address) (*ngtypes.Contract, error) {
	contractBucket := txn.Bucket(storage.ContractBucketName)

	rawAcc := contractBucket.Get(addr[:])
	if rawAcc == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no contract slot on %s", addr)
	}

	var acc ngtypes.Contract
	err := rlp.DecodeBytes(rawAcc, &acc)
	if err != nil {
		return nil, err
	}

	return &acc, nil
}

func contractExists(txn *bbolt.Tx, addr ngtypes.Address) bool {
	return txn.Bucket(storage.ContractBucketName).Get(addr[:]) != nil
}

func setContract(txn *bbolt.Tx, account *ngtypes.Contract) error {
	rawAccount, err := rlp.EncodeToBytes(account)
	if err != nil {
		return err
	}

	contractBucket := txn.Bucket(storage.ContractBucketName)
	err = contractBucket.Put(account.Owner[:], rawAccount)
	if err != nil {
		return errors.Wrap(err, "cannot set account")
	}

	return nil
}

func delContract(txn *bbolt.Tx, addr ngtypes.Address) error {
	return txn.Bucket(storage.ContractBucketName).Delete(addr[:])
}

func getBalance(txn *bbolt.Tx, addr ngtypes.Address) *big.Int {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	rawBalance := addr2balBucket.Get(addr[:])
	if rawBalance == nil {
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(rawBalance)
}

func setBalance(txn *bbolt.Tx, addr ngtypes.Address, balance *big.Int) error {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	err := addr2balBucket.Put(addr[:], balance.Bytes())
	if err != nil {
		return errors.Wrapf(err, "failed to set balance")
	}

	return nil
}

// keyResolver builds the compact-envelope resolver over the on-chain
// key registry of this txn
func keyResolver(txn *bbolt.Tx) ngtypes.PubKeyResolver {
	bucket := txn.Bucket(storage.KeyRegistryBucketName)

	return func(addr ngtypes.Address) []byte {
		return bucket.Get(addr[:])
	}
}

// registerPubKey records the address -> (scheme ‖ public key) binding
// a verified full-envelope tx revealed, enabling compact envelopes
// afterwards
func registerPubKey(txn *bbolt.Tx, tx *ngtypes.FullTx) error {
	if tx.IsCompactEnvelope() {
		return nil
	}

	scheme := tx.EnvelopeScheme()
	pkLen := ngtypes.PubKeySize(scheme)
	if pkLen == 0 || len(tx.Sign) != 2+pkLen+ngtypes.SigSize(scheme) {
		return nil
	}

	pubKey := tx.Sign[2 : 2+pkLen]
	addr := ngtypes.AddressOfPubKey(scheme, pubKey)

	bucket := txn.Bucket(storage.KeyRegistryBucketName)
	if bucket.Get(addr[:]) != nil {
		return nil // already registered
	}

	entry := append([]byte{byte(scheme)}, pubKey...)
	return bucket.Put(addr[:], entry)
}
