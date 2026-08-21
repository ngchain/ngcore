package ngblocks

import (
	"bytes"
	"encoding/binary"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// addrTxKey builds the account-history index key inside the tx bucket:
// atx: ‖ addr(32) ‖ heightBE(8) ‖ txHash(32). The height is BIG-endian on
// purpose: bbolt orders keys bytewise, so big-endian makes the on-disk order
// match numeric height order — which the range seek and iteration rely on
// (little-endian would interleave heights once they exceed one byte).
func addrTxKey(addr ngtypes.Address, height uint64, txHash []byte) []byte {
	var h [8]byte
	binary.BigEndian.PutUint64(h[:], height)

	key := make([]byte, 0, len(storage.AddrTxPrefix)+ngtypes.AddressSize+8+len(txHash))
	key = append(key, storage.AddrTxPrefix...)
	key = append(key, addr[:]...)
	key = append(key, h[:]...)
	key = append(key, txHash...)
	return key
}

// txAddresses returns the (deduped, non-zero) addresses a tx touches: its
// recipient and its derivable sender. From() is resolver-free for every
// envelope, so this works without the key registry
func txAddresses(tx *ngtypes.FullTx) []ngtypes.Address {
	var addrs []ngtypes.Address
	zero := ngtypes.Address{}

	if tx.To != zero {
		addrs = append(addrs, tx.To)
	}
	if from, err := tx.From(); err == nil && from != zero && from != tx.To {
		addrs = append(addrs, from)
	}
	return addrs
}

// putTxAddrIndex indexes a tx under each address it touches, at its height
func putTxAddrIndex(txBucket *bbolt.Bucket, tx *ngtypes.FullTx) error {
	hash := tx.GetHash()
	for _, addr := range txAddresses(tx) {
		if err := txBucket.Put(addrTxKey(addr, tx.Height, hash), nil); err != nil {
			return err
		}
	}
	return nil
}

// delTxAddrIndex removes a tx's account-history entries (reorg/prune)
func delTxAddrIndex(txBucket *bbolt.Bucket, tx *ngtypes.FullTx) error {
	hash := tx.GetHash()
	for _, addr := range txAddresses(tx) {
		if err := txBucket.Delete(addrTxKey(addr, tx.Height, hash)); err != nil {
			return err
		}
	}
	return nil
}

// GetTxsByAddress returns the txs an address touched, in height order,
// within [fromHeight, toHeight] and capped at limit (0 = a sane default).
// The genesis/checkpoint-origin block's txs are not indexed (they bypass
// putTxs), so an origin coinbase is not returned — acceptable, as genesis
// carries no premine.
func GetTxsByAddress(txBucket *bbolt.Bucket, addr ngtypes.Address, fromHeight, toHeight uint64, limit int) ([]*ngtypes.FullTx, error) {
	if limit <= 0 || limit > maxAddrTxLimit {
		limit = maxAddrTxLimit
	}
	if toHeight == 0 {
		toHeight = ^uint64(0)
	}

	prefix := append(append([]byte{}, storage.AddrTxPrefix...), addr[:]...)
	seek := addrTxKey(addr, fromHeight, nil)

	var out []*ngtypes.FullTx
	c := txBucket.Cursor()
	for k, _ := c.Seek(seek); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		if len(k) != len(storage.AddrTxPrefix)+ngtypes.AddressSize+8+32 {
			continue
		}
		height := binary.BigEndian.Uint64(k[len(storage.AddrTxPrefix)+ngtypes.AddressSize:])
		if height > toHeight {
			break
		}
		txHash := k[len(k)-32:]
		tx, err := GetTxByHash(txBucket, txHash)
		if err != nil {
			return nil, err
		}
		out = append(out, tx)
		if len(out) >= limit {
			break
		}
	}

	return out, nil
}

const maxAddrTxLimit = 1000

// PruneAddrTxIndexTxn drops account-history entries settled below floor —
// called on non-archive nodes alongside the receipt prune, so the index
// (like receipts) stays bounded instead of growing forever
func PruneAddrTxIndexTxn(txBucket *bbolt.Bucket, floor uint64) error {
	prefixLen := len(storage.AddrTxPrefix)
	c := txBucket.Cursor()
	for k, _ := c.Seek(storage.AddrTxPrefix); k != nil && bytes.HasPrefix(k, storage.AddrTxPrefix); k, _ = c.Next() {
		if len(k) != prefixLen+ngtypes.AddressSize+8+32 {
			continue
		}
		height := binary.BigEndian.Uint64(k[prefixLen+ngtypes.AddressSize:])
		if height < floor {
			if err := c.Delete(); err != nil {
				return err
			}
		}
	}
	return nil
}
