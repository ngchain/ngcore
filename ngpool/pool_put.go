package ngpool

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (pool *TxPool) PutNewTxFromLocal(tx *ngtypes.FullTx) (err error) {
	log.Debugf("putting new tx %x from rpc", tx.GetHash())

	err = pool.PutTx(tx)
	if err != nil {
		return err
	}

	err = pool.localNode.BroadcastTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutNewTxFromRemote puts tx from local(rpc) into txpool.
func (pool *TxPool) PutNewTxFromRemote(tx *ngtypes.FullTx) (err error) {
	log.Debugf("putting new tx %x from p2p", tx.GetHash())

	err = pool.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

var (
	ErrTxInvalidHeight = errors.New("invalid tx height")
	ErrPoolFull        = errors.New("tx pool is full")
	ErrTxFeeTooLow     = errors.New("tx fee does not beat the current pool entry")
)

// PutTx puts txs from network(p2p) or RPC into txpool, should check error before putting.
func (pool *TxPool) PutTx(tx *ngtypes.FullTx) error {
	pool.Lock()
	defer pool.Unlock()

	err := pool.db.View(func(txn *bbolt.Tx) error {
		if err := ngstate.CheckTx(txn, tx); err != nil {
			return errors.Wrap(err, "malformed tx, rejected")
		}

		return nil
	})
	if err != nil {
		return err
	}

	// txs are height-locked to the NEXT block: anything else can never
	// be packed (the pool resets on every tip change)
	nextHeight := pool.chain.GetLatestBlockHeight() + 1
	if tx.Height != nextHeight {
		return errors.Wrapf(ErrTxInvalidHeight, "tx %x is locked on height %d, the next block is %d",
			tx.GetHash(), tx.Height, nextHeight)
	}

	sender, err := tx.Sender()
	if err != nil {
		return err
	}

	// same-sender replacement: only a higher fee replaces
	if existing := pool.txMap[sender]; existing != nil {
		if existing.Fee.Cmp(tx.Fee) >= 0 {
			return errors.Wrapf(ErrTxFeeTooLow, "sender %s already queues a tx with fee %s",
				sender, existing.Fee)
		}
		pool.txMap[sender] = tx

		return nil
	}

	// capacity: when full, the new tx must beat the cheapest entry
	if len(pool.txMap) >= pool.MaxSize {
		evictAddr, evictFee := cheapestEntry(pool.txMap)
		if evictFee.Cmp(tx.Fee) >= 0 {
			return errors.Wrapf(ErrPoolFull, "pool holds %d txs and the cheapest fee %s beats %s",
				len(pool.txMap), evictFee, tx.Fee)
		}
		delete(pool.txMap, evictAddr)
	}

	pool.txMap[sender] = tx

	return nil
}

// cheapestEntry finds the pool entry with the lowest fee (the higher
// convener num breaks the tie, mirroring the pack order)
func cheapestEntry(txMap map[ngtypes.Address]*ngtypes.FullTx) (ngtypes.Address, *big.Int) {
	var addr ngtypes.Address
	var fee *big.Int

	for a, tx := range txMap {
		if fee == nil {
			addr, fee = a, tx.Fee
			continue
		}
		switch tx.Fee.Cmp(fee) {
		case -1:
			addr, fee = a, tx.Fee
		case 0:
			if bytes.Compare(a[:], addr[:]) > 0 {
				addr, fee = a, tx.Fee
			}
		}
	}

	return addr, fee
}
