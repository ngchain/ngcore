package ngpool

import (
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

	convener := uint64(tx.Convener)

	// same-convener replacement: only a higher fee replaces
	if existing := pool.txMap[convener]; existing != nil {
		if existing.Fee.Cmp(tx.Fee) >= 0 {
			return errors.Wrapf(ErrTxFeeTooLow, "convener %d already queues a tx with fee %s",
				convener, existing.Fee)
		}
		pool.txMap[convener] = tx

		return nil
	}

	// capacity: when full, the new tx must beat the cheapest entry
	if len(pool.txMap) >= pool.MaxSize {
		evictNum, evictFee := cheapestEntry(pool.txMap)
		if evictFee.Cmp(tx.Fee) >= 0 {
			return errors.Wrapf(ErrPoolFull, "pool holds %d txs and the cheapest fee %s beats %s",
				len(pool.txMap), evictFee, tx.Fee)
		}
		delete(pool.txMap, evictNum)
	}

	pool.txMap[convener] = tx

	return nil
}

// cheapestEntry finds the pool entry with the lowest fee (the higher
// convener num breaks the tie, mirroring the pack order)
func cheapestEntry(txMap map[uint64]*ngtypes.FullTx) (uint64, *big.Int) {
	var num uint64
	var fee *big.Int

	for n, tx := range txMap {
		if fee == nil {
			num, fee = n, tx.Fee
			continue
		}
		switch tx.Fee.Cmp(fee) {
		case -1:
			num, fee = n, tx.Fee
		case 0:
			if n > num {
				num, fee = n, tx.Fee
			}
		}
	}

	return num, fee
}
