package ngstate

import (
	"encoding/hex"
	"strings"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// TestReceiptMarshalJSON pins the rpc encoding of receipts: addresses as
// bs58, raw bytes as lowercase hex — never Go's default base64
func TestReceiptMarshalJSON(t *testing.T) {
	addr := testAddr(0xaa)

	event := Event{
		Contract: addr.Bytes(),
		Topic:    "transfer",
		Data:     []byte{0xde, 0xad},
	}
	raw, err := utils.JSON.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, addr.String()) {
		t.Fatalf("event json misses the bs58 address: %s", s)
	}
	if !strings.Contains(s, hex.EncodeToString(event.Data)) {
		t.Fatalf("event json misses the hex data: %s", s)
	}

	run := ContractRun{
		Contract: addr.Bytes(),
		Entry:    VMEntryOnTx,
		Ok:       false,
		Error:    "boom",
		GasUsed:  42,
		Events:   []Event{event},
	}
	raw, err = utils.JSON.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	s = string(raw)
	for _, want := range []string{addr.String(), `"entry":"ng:main"`, `"error":"boom"`, `"gasUsed":42`} {
		if !strings.Contains(s, want) {
			t.Fatalf("run json misses %q: %s", want, s)
		}
	}
}

// TestPruneReceipts: receipts age out past the retention window, broken
// records are swept with them, and recent receipts survive
func TestPruneReceipts(t *testing.T) {
	db := newTestDB(t)

	oldHash := []byte("tx-old")
	newHash := []byte("tx-new")
	brokenHash := []byte("tx-broken")

	err := db.Update(func(txn *bbolt.Tx) error {
		tip := uint64(receiptRetention + 100)

		if err := appendContractRun(txn, oldHash, 1, ContractRun{Ok: true}); err != nil {
			return err
		}
		if err := appendContractRun(txn, newHash, tip, ContractRun{Ok: true}); err != nil {
			return err
		}
		// an undecodable record must be swept too
		if err := txn.Bucket(storage.ReceiptBucketName).Put(brokenHash, []byte{0xff, 0xfe}); err != nil {
			return err
		}

		// a shallow tip prunes nothing
		if err := PruneReceiptsTxn(txn, receiptRetention); err != nil {
			return err
		}
		if runs, _ := GetTxRuns(txn, oldHash); len(runs) != 1 {
			t.Fatal("shallow prune must keep everything")
		}

		if err := PruneReceiptsTxn(txn, tip); err != nil {
			return err
		}

		if runs, _ := GetTxRuns(txn, oldHash); runs != nil {
			t.Fatalf("aged-out receipt survived: %+v", runs)
		}
		if raw := txn.Bucket(storage.ReceiptBucketName).Get(brokenHash); raw != nil {
			t.Fatal("broken receipt record survived the prune")
		}
		runs, err := GetTxRuns(txn, newHash)
		if err != nil {
			return err
		}
		if len(runs) != 1 || !runs[0].Ok {
			t.Fatalf("recent receipt lost: %+v", runs)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGetTxRunsErrors: a broken stored record errors out instead of
// returning garbage; appending onto it fails the same way
func TestGetTxRunsErrors(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		hash := []byte("tx-bad")
		if err := txn.Bucket(storage.ReceiptBucketName).Put(hash, []byte{0xff}); err != nil {
			return err
		}

		if _, err := GetTxRuns(txn, hash); err == nil {
			t.Fatal("broken receipt record must error")
		}
		if err := appendContractRun(txn, hash, 1, ContractRun{}); err == nil {
			t.Fatal("appending onto a broken record must error")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDeleteReceiptsAbove pins the reorg cleanup: receipts settled above
// the fork height are dropped (so re-applying the branch rebuilds them
// fresh instead of doubling runs), while receipts at or below survive
func TestDeleteReceiptsAbove(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		below := []byte("tx-below")
		above := []byte("tx-above")
		if err := appendContractRun(txn, below, 3, ContractRun{Ok: true}); err != nil {
			return err
		}
		if err := appendContractRun(txn, above, 7, ContractRun{Ok: true}); err != nil {
			return err
		}

		if err := deleteReceiptsAboveTxn(txn, 5); err != nil {
			return err
		}

		if runs, _ := GetTxRuns(txn, above); runs != nil {
			t.Fatalf("receipt above the fork survived: %+v", runs)
		}
		if runs, _ := GetTxRuns(txn, below); len(runs) != 1 {
			t.Fatal("receipt at/below the fork must be kept")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestStateGetTxRuns covers the State-level rpc reader and the error
// message truncation on stored runs
func TestStateGetTxRuns(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET, DB: db}

	hash := []byte("tx-rpc")
	hugeErr := strings.Repeat("e", maxRunErrorLen+100)

	err := db.Update(func(txn *bbolt.Tx) error {
		return appendContractRun(txn, hash, 1, ContractRun{Error: hugeErr})
	})
	if err != nil {
		t.Fatal(err)
	}

	runs, err := state.GetTxRuns(hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if len(runs[0].Error) != maxRunErrorLen {
		t.Fatalf("stored error length = %d, want truncated to %d", len(runs[0].Error), maxRunErrorLen)
	}

	// a tx which ran nothing has no receipt
	runs, err = state.GetTxRuns([]byte("tx-none"))
	if err != nil || runs != nil {
		t.Fatalf("unknown tx: runs=%v err=%v", runs, err)
	}
}

// TestTraceCallMarshalJSON pins the rpc encoding of a trace frame: bs58
// addresses, hex bytes
func TestTraceCallMarshalJSON(t *testing.T) {
	addr := testAddr(0xaa)
	tc := TraceCall{Type: "transfer", Depth: 1, From: addr.Bytes(), To: addr.Bytes(), Value: []byte{0x01, 0x02}, Ok: true}
	raw, err := utils.JSON.Marshal(tc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{addr.String(), `"type":"transfer"`, `"depth":1`, `"ok":true`, "0102"} {
		if !strings.Contains(s, want) {
			t.Fatalf("trace json misses %q: %s", want, s)
		}
	}
}

// TestReceiptRetainedFloorArchive: an archive node keeps every receipt, so
// the floor is 0 (and it does not even touch the db)
func TestReceiptRetainedFloorArchive(t *testing.T) {
	state := &State{Network: ngtypes.ZERONET, Archive: true}
	floor, err := state.ReceiptRetainedFloor()
	if err != nil || floor != 0 {
		t.Fatalf("archive floor = %d (%v), want 0", floor, err)
	}
}
