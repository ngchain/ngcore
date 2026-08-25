package ngpool_test

import (
	"bytes"
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// testEnv boots a chain with two funded, registered accounts:
// account 500 owned by keyA and account 600 owned by keyB
type testEnv struct {
	db    *bbolt.DB
	chain *blockchain.Chain
	pool  *ngpool.TxPool
	keyA  *ngtypes.PrivateKey
	keyB  *ngtypes.PrivateKey
}

// poolTestSalt is the fixed reveal nonce these tests use
var poolTestSalt = []byte("ngpool-test-salt")

// commitOnChain records the commitment a reveal tx needs, in the commit
// bucket at the current tip height (in window and strictly earlier than the
// reveal's next-block height), so PutTx's CheckTx admits the reveal.
func commitOnChain(t *testing.T, env *testEnv, owner *ngtypes.PrivateKey, tx *ngtypes.FullTx) {
	t.Helper()

	from := ngtypes.NewAddress(owner)
	buf := append(append([]byte{}, tx.UnheightedHash()...), tx.Salt...)
	hash := utils.Hash256(buf)

	tip := env.chain.GetLatestBlockHeight()
	key := make([]byte, 8+ngtypes.HashSize)
	copy(key[:8], utils.PackUint64LE(tip))
	copy(key[8:], hash)

	err := env.db.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.CommitBucketName).Put(key, from[:])
	})
	if err != nil {
		t.Fatalf("commitOnChain: %v", err)
	}
}

func mineWithTxs(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))
	block.SetCoinbase(ngtypes.NewAddress(miner))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}

	if err := block.ToUnsealing(append([]*ngtypes.FullTx{genTx}, txs...)); err != nil {
		t.Fatal(err)
	}

	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			return block
		}
	}

	t.Fatal("failed to seal a ZERONET block")
	return nil
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "pool.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	chain := blockchain.Init(db, ngtypes.ZERONET, store, state)
	pool := ngpool.Init(db, chain, nil)
	pool.MinFeePerByte = nil // raw-unit fees in these tests; the floor has its own test
	chain.OnTipChanged = pool.Reset

	keyA, _ := ngtypes.GenerateKey()
	keyB, _ := ngtypes.GenerateKey()

	// fund both keys: addresses spend directly, no registration
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineWithTxs(t, genesis, keyA)
	b2 := mineWithTxs(t, b1, keyB)

	for _, b := range []*ngtypes.FullBlock{b1, b2} {
		if err := chain.ApplyBlock(b); err != nil {
			t.Fatalf("apply block@%d: %v", b.GetHeight(), err)
		}
	}

	return &testEnv{db: db, chain: chain, pool: pool, keyA: keyA, keyB: keyB}
}

// transactTx builds a signed transact (reveal) tx from the owner's address
// and, when it is locked on the next block, seeds its commitment on chain so
// PutTx admits it. Wrong-height txs skip the commit (they are rejected at the
// height gate before the reveal check).
func transactTx(t *testing.T, env *testEnv, height uint64, owner *ngtypes.PrivateKey, fee int64) *ngtypes.FullTx {
	t.Helper()

	dest := testAddr()
	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
		dest, big.NewInt(10), big.NewInt(fee), nil, nil)
	tx.Salt = poolTestSalt
	if err := tx.Signature(owner); err != nil {
		t.Fatal(err)
	}

	if height == env.chain.GetLatestBlockHeight()+1 {
		commitOnChain(t, env, owner, tx)
	}
	return tx
}

func testAddr() ngtypes.Address {
	var addr ngtypes.Address
	addr[0] = 0xee
	return addr
}

func TestPutTxHeightGating(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1 // 5

	// the next block's height is the only admissible lock
	if err := env.pool.PutTx(transactTx(t, env, next, env.keyA, 1)); err != nil {
		t.Fatalf("tx locked on the next height rejected: %v", err)
	}

	for _, h := range []uint64{next - 1, next + 1} {
		err := env.pool.PutTx(transactTx(t, env, h, env.keyB, 1))
		if !errors.Is(err, ngpool.ErrTxInvalidHeight) {
			t.Fatalf("tx locked on height %d: got %v, want ErrTxInvalidHeight", h, err)
		}
	}
}

func TestPutTxReplacementByFee(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	cheap := transactTx(t, env, next, env.keyA, 1)
	rich := transactTx(t, env, next, env.keyA, 5)

	if err := env.pool.PutTx(cheap); err != nil {
		t.Fatal(err)
	}

	// same fee (the same tx again) and lower fees must not replace
	if err := env.pool.PutTx(cheap); !errors.Is(err, ngpool.ErrTxFeeTooLow) {
		t.Fatalf("same-fee replacement: got %v, want ErrTxFeeTooLow", err)
	}

	// a higher fee replaces
	if err := env.pool.PutTx(rich); err != nil {
		t.Fatalf("higher-fee replacement rejected: %v", err)
	}

	pack := env.pool.GetPack(next)
	if len(pack) != 1 || pack[0].Fee.Cmp(rich.Fee) != 0 {
		t.Fatalf("pool should hold the rich tx only, got %d txs", len(pack))
	}
}

func TestGetPackFeeSelection(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	txA := transactTx(t, env, next, env.keyA, 1) // cheap
	txB := transactTx(t, env, next, env.keyB, 9) // rich

	if err := env.pool.PutTx(txA); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.PutTx(txB); err != nil {
		t.Fatal(err)
	}

	// both fit: the pack carries them in canonical (tx hash) order
	pack := env.pool.GetPack(next)
	if len(pack) != 2 {
		t.Fatalf("pack size = %d, want 2", len(pack))
	}
	if bytes.Compare(pack[0].GetHash(), pack[1].GetHash()) > 0 {
		t.Fatal("pack must be in canonical tx-hash order")
	}

	// wrong height packs nothing
	if empty := env.pool.GetPack(next + 1); len(empty) != 0 {
		t.Fatalf("pack for a future height should be empty, got %d", len(empty))
	}
}

func TestPoolCapacityEviction(t *testing.T) {
	env := newTestEnv(t)
	env.pool.MaxSize = 1
	next := env.chain.GetLatestBlockHeight() + 1

	cheap := transactTx(t, env, next, env.keyA, 1)
	rich := transactTx(t, env, next, env.keyB, 9)

	if err := env.pool.PutTx(cheap); err != nil {
		t.Fatal(err)
	}

	// the richer newcomer evicts the cheapest entry
	if err := env.pool.PutTx(rich); err != nil {
		t.Fatalf("rich tx should evict the cheap one: %v", err)
	}

	pack := env.pool.GetPack(next)
	if len(pack) != 1 || !bytes.Equal(pack[0].GetHash(), rich.GetHash()) {
		t.Fatal("pool should hold the rich tx only")
	}

	// a newcomer which cannot outbid is rejected
	if err := env.pool.PutTx(cheap); !errors.Is(err, ngpool.ErrPoolFull) {
		t.Fatalf("got %v, want ErrPoolFull", err)
	}
}

func TestPoolResetOnTipChange(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	if err := env.pool.PutTx(transactTx(t, env, next, env.keyA, 1)); err != nil {
		t.Fatal(err)
	}
	if len(env.pool.GetPack(next)) != 1 {
		t.Fatal("pool should hold the queued tx")
	}

	// a new block moves the tip: the hook wipes the height-locked pool
	tip := env.chain.GetLatestBlock().(*ngtypes.FullBlock)
	if err := env.chain.ApplyBlock(mineWithTxs(t, tip, env.keyA)); err != nil {
		t.Fatal(err)
	}

	if len(env.pool.GetPack(next)) != 0 {
		t.Fatal("tip change must deprecate the pool")
	}
}

// TestRelayFeeFloor: the pool prices admission per wire byte; a tx
// below the floor never relays, one at the floor does
func TestRelayFeeFloor(t *testing.T) {
	env := newTestEnv(t)
	env.pool.MinFeePerByte = big.NewInt(1000)
	next := env.chain.GetLatestBlockHeight() + 1

	cheap := transactTx(t, env, next, env.keyA, 1)
	if err := env.pool.PutTx(cheap); !errors.Is(err, ngpool.ErrTxFeeBelowFloor) {
		t.Fatalf("got %v, want ErrTxFeeBelowFloor", err)
	}

	// pay comfortably above the floor (fee = 1000 * 10KB covers any envelope)
	rich := transactTx(t, env, next, env.keyA, 10_000_000)
	if err := env.pool.PutTx(rich); err != nil {
		t.Fatalf("floor-clearing tx refused: %v", err)
	}
}
