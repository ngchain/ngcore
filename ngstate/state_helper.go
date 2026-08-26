package ngstate

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// storedContract is the on-disk shape of a contract slot: the code
// lives ONCE in the code bucket, addressed by its blake3 hash, so
// identical modules deployed by many addresses cost one copy. The slot
// only references the hash
type storedContract struct {
	Owner    ngtypes.Address
	CodeHash []byte
	Context  *ngtypes.ContractContext
}

// getContract loads the contract slot of an address, resolving its code
// from the content-addressed code bucket; ErrKeyNotFound when the
// address never deployed
func getContract(txn *bbolt.Tx, addr ngtypes.Address) (*ngtypes.Contract, error) {
	raw := txn.Bucket(storage.ContractBucketName).Get(addr[:])
	if raw == nil {
		// lazy fork: a local miss may resolve remotely (no-op on nodes)
		if acc, _, ok := fetchRemoteState(txn, addr); ok && acc != nil {
			return acc, nil
		}
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no contract slot on %s", addr)
	}

	return decodeStoredContract(txn, raw)
}

// decodeStoredContract turns a raw contract-slot blob into a Contract,
// resolving its code from the content-addressed code bucket. Shared by
// the current-state reader and the archive historical resolver
func decodeStoredContract(txn *bbolt.Tx, raw []byte) (*ngtypes.Contract, error) {
	var sc storedContract
	if err := rlp.DecodeBytes(raw, &sc); err != nil {
		return nil, err
	}

	return &ngtypes.Contract{
		Owner:   sc.Owner,
		Source:  loadCode(txn, sc.CodeHash),
		Context: sc.Context,
	}, nil
}

func contractExists(txn *bbolt.Tx, addr ngtypes.Address) bool {
	if txn.Bucket(storage.ContractBucketName).Get(addr[:]) != nil {
		return true
	}
	// lazy fork: a local miss may resolve remotely (no-op on nodes)
	acc, _, ok := fetchRemoteState(txn, addr)
	return ok && acc != nil
}

// setContract stores the slot and reconciles the code registry: the
// new module is retained (a fresh hash is stored once, a shared hash
// just bumps its refcount) and any previously-referenced module is
// released
func setContract(txn *bbolt.Tx, rec *changeset, account *ngtypes.Contract) error {
	newHash := utils.Hash256(account.Source)

	rec.recordContract(txn, account.Owner) // pre-image before overwrite (archive)

	// release the code the slot referenced before, if it changed
	if prev := txn.Bucket(storage.ContractBucketName).Get(account.Owner[:]); prev != nil {
		var old storedContract
		if err := rlp.DecodeBytes(prev, &old); err == nil && !bytes.Equal(old.CodeHash, newHash) {
			releaseCode(txn, old.CodeHash, rec != nil)
		} else if err == nil && bytes.Equal(old.CodeHash, newHash) {
			// unchanged code: keep the refcount as-is
			goto store
		}
	}
	retainCode(txn, newHash, account.Source)

store:
	raw, err := rlp.EncodeToBytes(&storedContract{
		Owner:    account.Owner,
		CodeHash: newHash,
		Context:  account.Context,
	})
	if err != nil {
		return err
	}

	if err := txn.Bucket(storage.ContractBucketName).Put(account.Owner[:], raw); err != nil {
		return errors.Wrap(err, "cannot set contract slot")
	}

	// keep the state commitment in sync: the leaf hashes the exact stored blob
	trieSetContract(txn, account.Owner, raw)

	return nil
}

func delContract(txn *bbolt.Tx, rec *changeset, addr ngtypes.Address) error {
	bucket := txn.Bucket(storage.ContractBucketName)
	rec.recordContract(txn, addr) // pre-image before delete (archive)
	if prev := bucket.Get(addr[:]); prev != nil {
		var old storedContract
		if err := rlp.DecodeBytes(prev, &old); err == nil {
			releaseCode(txn, old.CodeHash, rec != nil)
		}
	}

	if err := bucket.Delete(addr[:]); err != nil {
		return err
	}
	// drop the contract leaf from the commitment
	trieSetContract(txn, addr, nil)
	return nil
}

// the code bucket entry is refcount(8 LE) ‖ wasm: one physical copy of
// each distinct module, shared by every slot that references it

func loadCode(txn *bbolt.Tx, codeHash []byte) []byte {
	entry := txn.Bucket(storage.CodeBucketName).Get(codeHash)
	if len(entry) < 8 {
		return nil
	}
	return entry[8:]
}

func retainCode(txn *bbolt.Tx, codeHash, code []byte) {
	bucket := txn.Bucket(storage.CodeBucketName)
	entry := bucket.Get(codeHash)
	if entry == nil {
		out := make([]byte, 8+len(code))
		binary.LittleEndian.PutUint64(out[:8], 1)
		copy(out[8:], code)
		_ = bucket.Put(codeHash, out)
		return
	}

	refs := binary.LittleEndian.Uint64(entry[:8]) + 1
	updated := append([]byte{}, entry...)
	binary.LittleEndian.PutUint64(updated[:8], refs)
	_ = bucket.Put(codeHash, updated)
}

// releaseCode drops one reference to a module. keepForHistory (set on
// archive nodes) retains the bytes even at refcount 0, so a historical
// contract slot that still references this hash can resolve its code;
// loadCode ignores the refcount, and a future deploy just re-retains it
func releaseCode(txn *bbolt.Tx, codeHash []byte, keepForHistory bool) {
	bucket := txn.Bucket(storage.CodeBucketName)
	entry := bucket.Get(codeHash)
	if len(entry) < 8 {
		return
	}

	refs := binary.LittleEndian.Uint64(entry[:8])
	if refs <= 1 && !keepForHistory {
		_ = bucket.Delete(codeHash) // last reference: reclaim the module
		return
	}

	updated := append([]byte{}, entry...)
	if refs > 0 {
		refs--
	}
	binary.LittleEndian.PutUint64(updated[:8], refs)
	_ = bucket.Put(codeHash, updated)
}

func getBalance(txn *bbolt.Tx, addr ngtypes.Address) *big.Int {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	rawBalance := addr2balBucket.Get(addr[:])
	if rawBalance == nil {
		// lazy fork: a local miss may resolve remotely (no-op on nodes)
		if _, bal, ok := fetchRemoteState(txn, addr); ok && bal != nil {
			return bal
		}
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(rawBalance)
}

func setBalance(txn *bbolt.Tx, rec *changeset, addr ngtypes.Address, balance *big.Int) error {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	rec.recordBal(txn, addr) // pre-image before overwrite (archive)

	err := addr2balBucket.Put(addr[:], balance.Bytes())
	if err != nil {
		return errors.Wrapf(err, "failed to set balance")
	}

	// keep the state commitment in sync (zero balance -> absent leaf)
	trieSetBalance(txn, addr, balance)

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
func registerPubKey(txn *bbolt.Tx, rec *changeset, tx *ngtypes.FullTx) error {
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

	rec.recordKey(txn, addr) // first reveal: record so a reorg can drop it

	entry := append([]byte{byte(scheme)}, pubKey...)
	if err := bucket.Put(addr[:], entry); err != nil {
		return err
	}
	// mirror the append-only key registry into the commitment
	trieSetKey(txn, addr, entry)
	return nil
}
