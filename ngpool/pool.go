package ngpool

import (
	"bytes"
	"math/big"
	"sync"

	logging "github.com/ngchain/zap-log"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngpool")

// DefaultPoolSize bounds how many conveners can queue a tx at once
const DefaultPoolSize = 4096

// TxPool is a little mem db which stores **signed** tx.
// RULE: One Account can only send one Tx (a higher fee replaces),
// txs are height-locked to the next block, and every tip movement
// deprecates the whole pool.
type TxPool struct {
	sync.Mutex

	db    *bbolt.DB
	txMap map[ngtypes.Address]*ngtypes.FullTx // From address -> queued reveal/effect tx

	// commitMap holds the pending blind commitments, keyed by their
	// committer. The one-tx-per-address rule is relaxed so an address may
	// queue one commit AND one reveal at once (a commit for a future tx can
	// ride the same block as the reveal of a prior one)
	commitMap map[ngtypes.Address]*ngtypes.Commitment

	// MaxSize caps the pool; when full, a new tx must outbid the
	// cheapest queued one
	MaxSize int

	// MinFeePerByte is the relay-policy fee floor: a tx must pay at
	// least MinFeePerByte * len(rlp(tx)) to enter this node's pool.
	// Local policy, not consensus — zero disables the floor
	MinFeePerByte *big.Int

	// OnNewTx, when set, fires after a tx successfully enters the pool
	// (drives the rpc pending-tx subscription). It runs under the pool
	// lock, so it MUST NOT call back into the pool
	OnNewTx func(*ngtypes.FullTx)

	// OnNewCommit, when set, fires after a commitment enters the pool. Same
	// contract as OnNewTx: runs under the pool lock, must not re-enter
	OnNewCommit func(*ngtypes.Commitment)

	chain     *blockchain.Chain
	localNode *ngp2p.LocalNode
}

func Init(db *bbolt.DB, chain *blockchain.Chain, localNode *ngp2p.LocalNode) *TxPool {
	pool := &TxPool{
		Mutex:     sync.Mutex{},
		db:        db,
		txMap:     make(map[ngtypes.Address]*ngtypes.FullTx),
		commitMap: make(map[ngtypes.Address]*ngtypes.Commitment),

		MaxSize:       DefaultPoolSize,
		MinFeePerByte: DefaultMinFeePerByte,

		chain:     chain,
		localNode: localNode,
	}

	return pool
}

// IsInPool checks one tx is in pool or not.
func (pool *TxPool) IsInPool(txHash []byte) (exists bool, inPoolTx *ngtypes.FullTx) {
	pool.Lock()
	defer pool.Unlock()

	for _, txInQueue := range pool.txMap {
		if bytes.Equal(txInQueue.GetHash(), txHash) {
			return true, txInQueue
		}
	}

	return false, nil
}

// List returns every tx currently queued in the pool (unordered). Used by
// the rpc mempool query; the pool holds at most one tx per From address.
func (pool *TxPool) List() []*ngtypes.FullTx {
	pool.Lock()
	defer pool.Unlock()

	txs := make([]*ngtypes.FullTx, 0, len(pool.txMap))
	for _, tx := range pool.txMap {
		txs = append(txs, tx)
	}

	return txs
}

// Reset cleans all txs AND commitments inside the pool. Both are
// height-locked to a single block, so a tip movement deprecates them all: a
// reveal stays admissible across the CommitWindow because its commitment is
// already on chain (CheckTx re-admits it against the committed state), not
// because the pool holds it.
func (pool *TxPool) Reset() {
	pool.Lock()
	defer pool.Unlock()

	pool.txMap = make(map[ngtypes.Address]*ngtypes.FullTx)
	pool.commitMap = make(map[ngtypes.Address]*ngtypes.Commitment)
}

// ListCommitments returns every commitment currently queued (unordered).
func (pool *TxPool) ListCommitments() []*ngtypes.Commitment {
	pool.Lock()
	defer pool.Unlock()

	commits := make([]*ngtypes.Commitment, 0, len(pool.commitMap))
	for _, c := range pool.commitMap {
		commits = append(commits, c)
	}

	return commits
}

// DefaultMinFeePerByte prices relay at 10 gigapico (1e10) per byte: a
// ~200-byte secp transfer costs ~0.000002 NG, a 7.9 KB hash-based
// envelope ~0.00008 NG — spam-hostile, human-negligible
var DefaultMinFeePerByte = big.NewInt(10_000_000_000)
