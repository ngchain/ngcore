package ngstate

import (
	"strings"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// spoofWat tries to emit under the reserved ng. namespace, then a normal
// topic: "ng.transfer" (offset 0, len 11) must be rejected, "ok" (offset
// 11, len 2) must pass
const spoofWat = `
(module
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "ng.transferok")
  (func (export "ng:main")
    (drop (call $emit (i32.const 0) (i32.const 11) (i32.const 0) (i32.const 0)))
    (drop (call $emit (i32.const 11) (i32.const 2) (i32.const 0) (i32.const 0)))))
`

// TestEmitReservedTopicRejected pins that a contract cannot forge a log in
// the ng. system namespace, so ng_getLogs consumers can trust ng.transfer
// logs are genuine node-emitted internal transfers
func TestEmitReservedTopicRejected(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}
	addr := testAddr(0xcd)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(spoofWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		tx := fakeTransactTx(ngtypes.Address{}, nil)
		state.runContract(txn, addr, tx, VMEntryOnTx, 1, nil)

		runs, err := GetTxRuns(txn, tx.GetHash())
		if err != nil {
			return err
		}
		if len(runs) != 1 {
			t.Fatalf("runs = %d, want 1", len(runs))
		}
		for _, ev := range runs[0].Events {
			if strings.HasPrefix(ev.Topic, EventTopicPrefix) {
				t.Fatalf("a contract forged a reserved topic: %q", ev.Topic)
			}
		}
		if len(runs[0].Events) != 1 || runs[0].Events[0].Topic != "ok" {
			t.Fatalf("events = %+v, want only [ok]", runs[0].Events)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
