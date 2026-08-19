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
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// transactTxV is transactTx with a chosen value: the txid hashes the tx
// WITHOUT its signature envelope, so same-content txs from different
// signers collide on the hash — a distinct value keeps txids apart
func transactTxV(t *testing.T, height uint64, owner *ngtypes.PrivateKey, value, fee int64) *ngtypes.FullTx {
	t.Helper()

	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
		testAddr(), big.NewInt(value), big.NewInt(fee), nil, nil)
	if err := tx.Signature(owner); err != nil {
		t.Fatal(err)
	}
	return tx
}

// fundNewKeys mines one block per new key so each key's address holds a
// block reward, then returns the keys
func fundNewKeys(t *testing.T, env *testEnv, n int) []*ngtypes.PrivateKey {
	t.Helper()

	keys := make([]*ngtypes.PrivateKey, 0, n)
	for i := 0; i < n; i++ {
		key, err := ngtypes.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}

		tip := env.chain.GetLatestBlock().(*ngtypes.FullBlock)
		if err := env.chain.ApplyBlock(mineWithTxs(t, tip, key)); err != nil {
			t.Fatal(err)
		}

		keys = append(keys, key)
	}

	return keys
}

func TestIsInPool(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	tx := transactTx(t, next, env.keyA, 1)

	// empty pool knows nothing
	if exists, got := env.pool.IsInPool(tx.GetHash()); exists || got != nil {
		t.Fatal("empty pool claims to hold the tx")
	}

	if err := env.pool.PutTx(tx); err != nil {
		t.Fatal(err)
	}

	exists, got := env.pool.IsInPool(tx.GetHash())
	if !exists || got == nil || !bytes.Equal(got.GetHash(), tx.GetHash()) {
		t.Fatal("pool must find the queued tx by its hash")
	}

	// an unknown hash misses even on a non-empty pool
	unknown := make([]byte, 32)
	unknown[0] = 0xff
	if exists, got := env.pool.IsInPool(unknown); exists || got != nil {
		t.Fatal("pool claims to hold an unknown hash")
	}
}

func TestPutTxRejectsMalformed(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	// unsigned tx must never enter the pool
	unsigned := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, next,
		testAddr(), big.NewInt(10), big.NewInt(1), nil, nil)
	if err := env.pool.PutTx(unsigned); !errors.Is(err, ngtypes.ErrTxSignInvalid) {
		t.Fatalf("unsigned tx: got %v, want ErrTxSignInvalid", err)
	}

	// a signed tx spending beyond the balance must be rejected
	tooRich := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, next,
		testAddr(), new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1), nil, nil)
	if err := tooRich.Signature(env.keyA); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.PutTx(tooRich); !errors.Is(err, ngstate.ErrTxrBalanceInsufficient) {
		t.Fatalf("overspending tx: got %v, want ErrTxrBalanceInsufficient", err)
	}

	// nothing malformed may linger in the pool
	if pack := env.pool.GetPack(next); len(pack) != 0 {
		t.Fatalf("pool must stay empty, got %d txs", len(pack))
	}
}

func TestPutNewTxFromRemote(t *testing.T) {
	env := newTestEnv(t)
	next := env.chain.GetLatestBlockHeight() + 1

	if err := env.pool.PutNewTxFromRemote(transactTx(t, next, env.keyA, 1)); err != nil {
		t.Fatalf("valid remote tx rejected: %v", err)
	}

	err := env.pool.PutNewTxFromRemote(transactTx(t, next+1, env.keyB, 1))
	if !errors.Is(err, ngpool.ErrTxInvalidHeight) {
		t.Fatalf("remote tx on a wrong height: got %v, want ErrTxInvalidHeight", err)
	}

	if pack := env.pool.GetPack(next); len(pack) != 1 {
		t.Fatalf("pool should hold the one valid remote tx, got %d", len(pack))
	}
}

// TestEvictionTieBreak: when the pool is full and several queued txs
// share the lowest fee, the entry with the LARGER address is evicted —
// deterministic whatever the map iteration order is
func TestEvictionTieBreak(t *testing.T) {
	env := newTestEnv(t)
	extra := fundNewKeys(t, env, 2)
	keyC, keyD := extra[0], extra[1]

	env.pool.MaxSize = 3
	next := env.chain.GetLatestBlockHeight() + 1

	txA := transactTxV(t, next, env.keyA, 10, 2) // safe: not the cheapest
	txB := transactTxV(t, next, env.keyB, 11, 1) // tied cheapest
	txC := transactTxV(t, next, keyC, 12, 1)     // tied cheapest
	txD := transactTxV(t, next, keyD, 13, 9)     // the outbidding newcomer

	addrB := ngtypes.NewAddress(env.keyB)
	addrC := ngtypes.NewAddress(keyC)

	evicted, kept := txB, txC
	if bytes.Compare(addrC[:], addrB[:]) > 0 {
		evicted, kept = txC, txB
	}

	// map iteration order shuffles per run: repeat so the eviction pick
	// proves deterministic, not lucky
	for i := 0; i < 40; i++ {
		env.pool.Reset()

		for _, tx := range []*ngtypes.FullTx{txA, txB, txC} {
			if err := env.pool.PutTx(tx); err != nil {
				t.Fatal(err)
			}
		}

		if err := env.pool.PutTx(txD); err != nil {
			t.Fatalf("outbidding newcomer rejected: %v", err)
		}

		if exists, _ := env.pool.IsInPool(evicted.GetHash()); exists {
			t.Fatal("the tied entry with the larger address must be evicted")
		}
		for _, tx := range []*ngtypes.FullTx{txA, kept, txD} {
			if exists, _ := env.pool.IsInPool(tx.GetHash()); !exists {
				t.Fatalf("tx with fee %s missing from the pool", tx.Fee)
			}
		}
	}
}

// TestGetPackOrderAndTies: the pack re-sorts the fee-picked survivors
// into canonical tx-hash order, fee ties included
func TestGetPackOrderAndTies(t *testing.T) {
	env := newTestEnv(t)
	extra := fundNewKeys(t, env, 2)
	next := env.chain.GetLatestBlockHeight() + 1

	txs := []*ngtypes.FullTx{
		transactTxV(t, next, env.keyA, 10, 9),
		transactTxV(t, next, env.keyB, 11, 5),
		transactTxV(t, next, extra[0], 12, 5), // ties with keyB's fee
		transactTxV(t, next, extra[1], 13, 1),
	}
	for _, tx := range txs {
		if err := env.pool.PutTx(tx); err != nil {
			t.Fatal(err)
		}
	}

	// the pack slice is rebuilt from the map every call: repeat so the
	// sort sees the txs in shuffled orders too
	for i := 0; i < 20; i++ {
		pack := env.pool.GetPack(next)
		if len(pack) != len(txs) {
			t.Fatalf("pack size = %d, want %d", len(pack), len(txs))
		}

		for j := 1; j < len(pack); j++ {
			if bytes.Compare(pack[j-1].GetHash(), pack[j].GetHash()) >= 0 {
				t.Fatal("pack must be in strict canonical tx-hash order")
			}
		}

		for _, tx := range txs {
			found := false
			for _, packed := range pack {
				if bytes.Equal(packed.GetHash(), tx.GetHash()) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("tx with fee %s missing from the pack", tx.Fee)
			}
		}
	}
}

// TestPutNewTxFromLocal drives the rpc entry: a valid tx enters the pool
// and broadcasts (to zero peers), a bad one fails before broadcasting
func TestPutNewTxFromLocal(t *testing.T) {
	dir := t.TempDir()

	db, err := bbolt.Open(filepath.Join(dir, "pool.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	chain := blockchain.Init(db, ngtypes.ZERONET, store, state)

	localNode := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:                  filepath.Join(dir, "ngp2p.key"),
		Network:                     ngtypes.ZERONET,
		Port:                        0, // any free port
		DisableDiscovery:            true,
		DisableConnectingBootstraps: true,
	})
	t.Cleanup(func() { _ = localNode.Close() })

	pool := ngpool.Init(db, chain, localNode)
	pool.MinFeePerByte = nil

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	if err := chain.ApplyBlock(mineWithTxs(t, genesis, key)); err != nil {
		t.Fatal(err)
	}

	next := chain.GetLatestBlockHeight() + 1

	tx := transactTx(t, next, key, 1)
	if err := pool.PutNewTxFromLocal(tx); err != nil {
		t.Fatalf("valid local tx rejected: %v", err)
	}
	if exists, _ := pool.IsInPool(tx.GetHash()); !exists {
		t.Fatal("local tx must land in the pool")
	}

	err = pool.PutNewTxFromLocal(transactTx(t, next+1, key, 1))
	if !errors.Is(err, ngpool.ErrTxInvalidHeight) {
		t.Fatalf("local tx on a wrong height: got %v, want ErrTxInvalidHeight", err)
	}
}
