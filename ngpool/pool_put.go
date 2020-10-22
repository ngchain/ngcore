package ngpool

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngstate"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (pool *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %x from rpc", tx.Hash())

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
	log.Debugf("putting new tx %x from p2p", tx.Hash())

	err = pool.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutTx puts txs from network(p2p) or RPC into txpool, should check error before putting.
// TODO: implement me
func (pool *TxPool) PutTx(tx *ngtypes.Tx) error {
	pool.Lock()
	defer pool.Unlock()

	err := pool.db.View(func(txn *badger.Txn) error {
		if err := ngstate.CheckTxs(txn, tx); err != nil {
			return fmt.Errorf("malformed tx, rejected: %v", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	latestBlock := pool.chain.GetLatestBlock()

	if !bytes.Equal(tx.PrevBlockHash, latestBlock.PrevBlockHash) {
		return fmt.Errorf("tx %x does not belong to current State, found %x, require %x",
			tx.Hash(), tx.PrevBlockHash, latestBlock.PrevBlockHash)
	}

	if pool.txMap[tx.Convener] == nil ||
		new(big.Int).SetBytes(pool.txMap[tx.Convener].Fee).Cmp(new(big.Int).SetBytes(tx.Fee)) < 0 {
		pool.txMap[tx.Convener] = tx
	}

	return nil
}
