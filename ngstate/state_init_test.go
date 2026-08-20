package ngstate

import (
	"bytes"
	"math/big"
	"sync"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// newTestState wires a State over a fresh test db with an empty
// in-memory snapshot manager
func newTestState(t *testing.T, db *bbolt.DB) *State {
	t.Helper()

	return &State{
		Network: ngtypes.ZERONET,
		DB:      db,
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
	}
}

func TestInitStateFromGenesis(t *testing.T) {
	db := newTestDB(t)

	state := InitStateFromGenesis(db, ngtypes.ZERONET)
	if state == nil {
		t.Fatal("no state")
	}

	// the genesis sheet's balances must be applied
	sheet := ngtypes.GetGenesisSheet(ngtypes.ZERONET)
	for _, bal := range sheet.Balances {
		got, err := state.GetTotalBalanceByAddress(bal.Address)
		if err != nil {
			t.Fatal(err)
		}
		if got.Cmp(bal.Amount) < 0 {
			t.Fatalf("genesis balance of %s = %s, want at least %s", bal.Address, got, bal.Amount)
		}
	}
}

func TestInitStateFromSheet(t *testing.T) {
	db := newTestDB(t)

	addr := testAddr(0x11)
	keyAddr := testAddr(0x12)
	code := mustWat(logWat)

	sheet := ngtypes.NewSheet(ngtypes.ZERONET, 8, []byte("sheet-hash"),
		[]*ngtypes.Balance{{Address: addr, Amount: big.NewInt(123)}},
		[]*ngtypes.Contract{ngtypes.NewContract(addr, code, nil)},
		[]*ngtypes.RegisteredKey{{Address: keyAddr, Entry: []byte{1, 2, 3}}},
	)

	state := InitStateFromSheet(db, ngtypes.ZERONET, sheet)
	state.Network = ngtypes.ZERONET

	err := state.View(func(txn *bbolt.Tx) error {
		if got := getBalance(txn, addr); got.Int64() != 123 {
			t.Fatalf("balance = %s, want 123", got)
		}
		acc, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if !bytes.Equal(acc.Source, code) {
			t.Fatal("contract source lost through the sheet")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if !state.PubKeyRegistered(keyAddr) {
		t.Fatal("the sheet's key registry row is lost")
	}
	if state.PubKeyRegistered(testAddr(0x13)) {
		t.Fatal("an unknown address must not be registered")
	}
}

// TestRebuildFromSheet: a rebuild wipes whatever state accumulated and
// re-applies exactly the sheet
func TestRebuildFromSheet(t *testing.T) {
	db := newTestDB(t)
	state := InitStateFromGenesis(db, ngtypes.ZERONET)
	state.Network = ngtypes.ZERONET

	junkAddr := testAddr(0x21)
	err := state.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, junkAddr, big.NewInt(999)); err != nil {
			return err
		}
		return setContract(txn, nil, ngtypes.NewContract(junkAddr, mustWat(logWat), nil))
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := state.RebuildFromSheet(ngtypes.GetGenesisSheet(ngtypes.ZERONET)); err != nil {
		t.Fatalf("RebuildFromSheet: %v", err)
	}

	err = state.View(func(txn *bbolt.Tx) error {
		if got := getBalance(txn, junkAddr); got.Sign() != 0 {
			t.Fatalf("junk balance survived the rebuild: %s", got)
		}
		if _, err := getContract(txn, junkAddr); err == nil {
			t.Fatal("junk contract survived the rebuild")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRebuildFromBlockStore replays the canonical chain (here: genesis)
// after wiping the state buckets
func TestRebuildFromBlockStore(t *testing.T) {
	db := newTestDB(t)
	ngblocks.Init(db, ngtypes.ZERONET)
	state := newTestState(t, db)

	junkAddr := testAddr(0x22)
	err := state.Update(func(txn *bbolt.Tx) error {
		return setBalance(txn, nil, junkAddr, big.NewInt(555))
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := state.RebuildFromBlockStore(); err != nil {
		t.Fatalf("RebuildFromBlockStore: %v", err)
	}

	err = state.View(func(txn *bbolt.Tx) error {
		if got := getBalance(txn, junkAddr); got.Sign() != 0 {
			t.Fatalf("junk balance survived the replay: %s", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestStateExternalReaders covers the rpc-facing State getters
func TestStateExternalReaders(t *testing.T) {
	db := newTestDB(t)
	ngblocks.Init(db, ngtypes.ZERONET)
	state := newTestState(t, db)

	addr := testAddr(0x31)
	code := mustWat(logWat)

	err := state.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(77)); err != nil {
			return err
		}
		return setContract(txn, nil, ngtypes.NewContract(addr, code, nil))
	})
	if err != nil {
		t.Fatal(err)
	}

	bal, err := state.GetTotalBalanceByAddress(addr)
	if err != nil || bal.Int64() != 77 {
		t.Fatalf("total balance = %v, err = %v", bal, err)
	}

	acc, err := state.GetContract(addr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(acc.Source, code) {
		t.Fatal("GetContract lost the source")
	}
	if _, err := state.GetContract(testAddr(0x32)); err == nil {
		t.Fatal("GetContract on an empty address must error")
	}

	if state.PubKeyRegistered(addr) {
		t.Fatal("no key registered yet")
	}
	err = state.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.KeyRegistryBucketName).Put(addr[:], []byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !state.PubKeyRegistered(addr) {
		t.Fatal("registered key not reported")
	}

	// the mature balance at height 0 falls back to the genesis sheet
	mature, err := state.GetMatureBalanceByAddress(addr)
	if err != nil {
		t.Fatalf("GetMatureBalanceByAddress: %v", err)
	}
	if mature.Sign() != 0 {
		t.Fatalf("mature balance of a fresh address = %s, want 0", mature)
	}
}
