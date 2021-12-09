package ngpool

import (
	"github.com/c0mm4nd/dbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (pool *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
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
func (pool *TxPool) PutNewTxFromRemote(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %x from p2p", tx.GetHash())

	err = pool.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

var ErrTxInvalidHeight = errors.New("invalid tx height")

// PutTx puts txs from network(p2p) or RPC into txpool, should check error before putting.
func (pool *TxPool) PutTx(tx *ngtypes.Tx) error {
	pool.Lock()
	defer pool.Unlock()

	err := pool.db.View(func(txn *dbolt.Tx) error {
		if err := ngstate.CheckTx(txn, tx); err != nil {
			return errors.Wrap(err, "malformed tx, rejected")
		}

		return nil
	})
	if err != nil {
		return err
	}

	latestBlock := pool.chain.GetLatestBlock()

	if tx.Height != latestBlock.Header.Height {
		return errors.Wrapf(ErrTxInvalidHeight, "tx %x does not belong to current State, found %d, require %d",
			tx.GetHash(), tx.Height, latestBlock.Header.Height)
	}

	if pool.txMap[uint64(tx.Convener)] == nil ||
		pool.txMap[uint64(tx.Convener)].Fee.Cmp(tx.Fee) < 0 {
		pool.txMap[uint64(tx.Convener)] = tx
	}

	return nil
}
