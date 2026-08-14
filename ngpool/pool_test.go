package ngpool_test

import (
	"encoding/binary"
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
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
	chain *blockchain.Chain
	pool  *ngpool.TxPool
	keyA  *btcec.PrivateKey
	keyB  *btcec.PrivateKey
}

func mineWithTxs(t *testing.T, parent *ngtypes.FullBlock, miner *btcec.PrivateKey, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height, 0,
		[]ngtypes.Address{ngtypes.NewAddress(miner)},
		[]*big.Int{ngtypes.GetBlockReward(height)},
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

func registerTx(t *testing.T, height uint64, owner *btcec.PrivateKey, num uint64) *ngtypes.FullTx {
	t.Helper()

	extra := make([]byte, 8)
	binary.LittleEndian.PutUint64(extra, num)

	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.RegisterTx, height, 1,
		[]ngtypes.Address{ngtypes.NewAddress(owner)},
		[]*big.Int{big.NewInt(0)},
		ngtypes.RegisterFee, extra, nil)
	if err := tx.Signature(owner); err != nil {
		t.Fatal(err)
	}
	return tx
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
	chain.OnTipChanged = pool.Reset

	keyA, _ := btcec.NewPrivateKey()
	keyB, _ := btcec.NewPrivateKey()

	// fund both keys, then register their accounts
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineWithTxs(t, genesis, keyA)
	b2 := mineWithTxs(t, b1, keyB)
	b3 := mineWithTxs(t, b2, keyA, registerTx(t, 3, keyA, 500))
	b4 := mineWithTxs(t, b3, keyB, registerTx(t, 4, keyB, 600))

	for _, b := range []*ngtypes.FullBlock{b1, b2, b3, b4} {
		if err := chain.ApplyBlock(b); err != nil {
			t.Fatalf("apply block@%d: %v", b.GetHeight(), err)
		}
	}

	return &testEnv{chain: chain, pool: pool, keyA: keyA, keyB: keyB}
}

// transactTx builds a signed transact tx from the given registered account
func transactTx(t *testing.T, height uint64, convener uint64, owner *btcec.PrivateKey, fee int64) *ngtypes.FullTx {
	t.Helper()

	dest := testAddr()
	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height, ngtypes.AccountNum(convener),
		[]ngtypes.Address{dest}, []*big.Int{big.NewInt(10)}, big.NewInt(fee), nil, nil)
	if err := tx.Signature(owner); err != nil {
		t.Fatal(err)
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
	if err := env.pool.PutTx(transactTx(t, next, 500, env.keyA, 1)); err != nil {
		t.Fatalf("tx locked on the next height rejected: %v", err)
	}

	for _, h := range []uint64{next - 1, next + 1} {
		err := env.pool.PutTx(transactTx(t, h, 600, env.keyB, 1))
		if !errors.Is(err, ngpool.ErrTxInvalidHeight) {
			t.Fatalf("tx locked on height %d: got %v, want ErrTxInvalidHeight", h, err)
		}
	}
}

func TestPutTxReplacementByFee(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	cheap := transactTx(t, next, 500, env.keyA, 1)
	rich := transactTx(t, next, 500, env.keyA, 5)

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

	txA := transactTx(t, next, 500, env.keyA, 1) // cheap
	txB := transactTx(t, next, 600, env.keyB, 9) // rich

	if err := env.pool.PutTx(txA); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.PutTx(txB); err != nil {
		t.Fatal(err)
	}

	// both fit: the pack carries them in canonical (convener) order
	pack := env.pool.GetPack(next)
	if len(pack) != 2 {
		t.Fatalf("pack size = %d, want 2", len(pack))
	}
	if pack[0].Convener != 500 || pack[1].Convener != 600 {
		t.Fatal("pack must be in canonical convener order")
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

	cheap := transactTx(t, next, 500, env.keyA, 1)
	rich := transactTx(t, next, 600, env.keyB, 9)

	if err := env.pool.PutTx(cheap); err != nil {
		t.Fatal(err)
	}

	// the richer newcomer evicts the cheapest entry
	if err := env.pool.PutTx(rich); err != nil {
		t.Fatalf("rich tx should evict the cheap one: %v", err)
	}

	pack := env.pool.GetPack(next)
	if len(pack) != 1 || pack[0].Convener != 600 {
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

	if err := env.pool.PutTx(transactTx(t, next, 500, env.keyA, 1)); err != nil {
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
