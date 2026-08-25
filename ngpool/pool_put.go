package ngpool

import (
	"bytes"
	"github.com/c0mm4nd/rlp"
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

// PutNewCommitmentFromLocal pools and broadcasts a commitment from local (rpc).
func (pool *TxPool) PutNewCommitmentFromLocal(commit *ngtypes.Commitment) (err error) {
	log.Debugf("putting new commitment %x from rpc", commit.Hash)

	if err = pool.PutCommitment(commit); err != nil {
		return err
	}

	return pool.localNode.BroadcastCommitment(commit)
}

// PutNewCommitmentFromRemote pools a commitment received over p2p.
func (pool *TxPool) PutNewCommitmentFromRemote(commit *ngtypes.Commitment) (err error) {
	log.Debugf("putting new commitment %x from p2p", commit.Hash)

	return pool.PutCommitment(commit)
}

var ErrCommitInvalidHeight = errors.New("invalid commitment height")

// PutCommitment validates a commitment and queues it (one pending commit per
// committer; a higher fee replaces). Commitments are height-locked to the
// next block, like txs.
func (pool *TxPool) PutCommitment(commit *ngtypes.Commitment) error {
	pool.Lock()
	defer pool.Unlock()

	nextHeight := pool.chain.GetLatestBlockHeight() + 1

	err := pool.db.View(func(txn *bbolt.Tx) error {
		return ngstate.CheckCommitment(txn, commit, nextHeight)
	})
	if err != nil {
		return errors.Wrap(err, "malformed commitment, rejected")
	}

	// relay fee floor: a commitment must clear MinFeePerByte * its wire size,
	// same policy as a tx. Free/dust commits are the cheap-spam and
	// commit-many/reveal-one straddle surface, so they are not relayed
	if pool.MinFeePerByte != nil && pool.MinFeePerByte.Sign() > 0 {
		raw, err := rlp.EncodeToBytes(commit)
		if err != nil {
			return err
		}
		floor := new(big.Int).Mul(pool.MinFeePerByte, big.NewInt(int64(len(raw))))
		if commit.Fee.Cmp(floor) < 0 {
			return errors.Wrapf(ErrTxFeeBelowFloor, "commit fee %s < floor %s for %d bytes",
				commit.Fee, floor, len(raw))
		}
	}

	from, err := commit.From()
	if err != nil {
		return err
	}

	// same-from replacement: only a higher fee replaces the pending commit
	if existing := pool.commitMap[from]; existing != nil {
		if existing.Fee.Cmp(commit.Fee) >= 0 {
			return errors.Wrapf(ErrTxFeeTooLow, "from %s already queues a commitment with fee %s",
				from, existing.Fee)
		}
	}

	pool.commitMap[from] = commit
	pool.notifyNewCommit(commit)

	return nil
}

func (pool *TxPool) notifyNewCommit(commit *ngtypes.Commitment) {
	if pool.OnNewCommit != nil {
		pool.OnNewCommit(commit)
	}
}

var (
	ErrTxInvalidHeight = errors.New("invalid tx height")
	ErrPoolFull        = errors.New("tx pool is full")
	ErrTxFeeTooLow     = errors.New("tx fee does not beat the current pool entry")
	ErrTxFeeBelowFloor = errors.New("tx fee is below the relay fee floor")
)

// PutTx puts txs from network(p2p) or RPC into txpool, should check error before putting.
func (pool *TxPool) PutTx(tx *ngtypes.FullTx) error {
	pool.Lock()
	defer pool.Unlock()

	// cheap, stateless gates first: a tx locked on the wrong height or below
	// the relay floor can never be packed, whatever its content
	// txs are height-locked to the NEXT block (the pool resets on every tip
	// change)
	nextHeight := pool.chain.GetLatestBlockHeight() + 1
	if tx.Height != nextHeight {
		return errors.Wrapf(ErrTxInvalidHeight, "tx %x is locked on height %d, the next block is %d",
			tx.GetHash(), tx.Height, nextHeight)
	}

	// the relay fee floor scales with the tx's wire size, so heavy
	// envelopes pay for the bytes they burden the network with
	if pool.MinFeePerByte != nil && pool.MinFeePerByte.Sign() > 0 {
		raw, err := rlp.EncodeToBytes(tx)
		if err != nil {
			return err
		}
		floor := new(big.Int).Mul(pool.MinFeePerByte, big.NewInt(int64(len(raw))))
		if tx.Fee.Cmp(floor) < 0 {
			return errors.Wrapf(ErrTxFeeBelowFloor, "fee %s < floor %s for %d bytes",
				tx.Fee, floor, len(raw))
		}
	}

	// the reveal must be a valid tx against current committed state: its
	// commitment must already be on chain (CheckTx runs checkReveal)
	err := pool.db.View(func(txn *bbolt.Tx) error {
		if err := ngstate.CheckTx(txn, tx); err != nil {
			return errors.Wrap(err, "malformed tx, rejected")
		}

		return nil
	})
	if err != nil {
		return err
	}

	from, err := tx.From()
	if err != nil {
		return err
	}

	// same-from replacement: only a higher fee replaces
	if existing := pool.txMap[from]; existing != nil {
		if existing.Fee.Cmp(tx.Fee) >= 0 {
			return errors.Wrapf(ErrTxFeeTooLow, "from %s already queues a tx with fee %s",
				from, existing.Fee)
		}
		pool.txMap[from] = tx
		pool.notifyNewTx(tx)

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

	pool.txMap[from] = tx
	pool.notifyNewTx(tx)

	return nil
}

func (pool *TxPool) notifyNewTx(tx *ngtypes.FullTx) {
	if pool.OnNewTx != nil {
		pool.OnNewTx(tx)
	}
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
