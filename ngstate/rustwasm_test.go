package ngstate

import (
	"encoding/binary"
	"os"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRustContract proves the full language pipeline: a contract
// written in Rust, compiled to wasm, runs on the chain unmodified.
// Skips when the ../ngswap Rust artifact is not built.
func TestRustContract(t *testing.T) {
	const path = "../../ngswap/contracts/counter/target/wasm32-unknown-unknown/release/counter.wasm"
	wasm, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("rust artifact not built (%v); run: cargo build --release --target wasm32-unknown-unknown", err)
	}

	// it must pass the chain's wasm validation
	if _, err := LoadContractWasm(wasm); err != nil {
		t.Fatalf("rust wasm rejected by the loader: %v", err)
	}

	db := newTestDB(t)
	addr := testAddr(0x77)

	callAdd := func(amount uint64) uint64 {
		var total uint64
		err := db.Update(func(txn *bbolt.Tx) error {
			c, err := getContract(txn, addr)
			if err != nil {
				c = ngtypes.NewContract(addr, wasm, nil)
				c.SetActive(true)
			}
			putContract(t, txn, c, 0)

			arg := make([]byte, 8)
			binary.LittleEndian.PutUint64(arg, amount)
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1, addr, nil, nil, arg, nil)

			vm, err := NewVM(txn, c, tx, 1)
			if err != nil {
				return err
			}
			if err := vm.Run(VMEntryOnTx); err != nil {
				return err
			}
			r, _ := getContract(txn, addr)
			total = binary.LittleEndian.Uint64(r.Context.Get("n"))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return total
	}

	if got := callAdd(5); got != 5 {
		t.Fatalf("after +5, total = %d, want 5", got)
	}
	if got := callAdd(37); got != 42 {
		t.Fatalf("after +37, total = %d, want 42", got)
	}
}
