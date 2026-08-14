package ngstate

import (
	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/storage"
)

// Receipts are LOCAL, non-consensus data: every node derives them
// deterministically by executing the chain, so they never enter block
// hashes. A tx's receipt is the list of contract runs it triggered.

// Event is one contract-emitted log entry
type Event struct {
	Contract uint64 // the account which emitted (the executing frame)
	Topic    string
	Data     []byte
}

// ContractRun records one contract execution triggered by a tx
type ContractRun struct {
	Account uint64 // the contract the entry ran on
	Entry   string // main / init
	Ok      bool
	Error   string
	GasUsed uint64
	Events  []Event
}

// emission limits keep the local receipt store abuse-resistant
const (
	maxEventsPerRun  = 128
	maxEventTopicLen = 64
	maxEventDataLen  = 4096
	maxRunErrorLen   = 512
)

// appendContractRun attaches a run record to the tx's receipt
func appendContractRun(txn *bbolt.Tx, txHash []byte, run ContractRun) error {
	bucket := txn.Bucket(storage.ReceiptBucketName)

	var runs []ContractRun
	if raw := bucket.Get(txHash); raw != nil {
		if err := rlp.DecodeBytes(raw, &runs); err != nil {
			return errors.Wrap(err, "broken receipt record")
		}
	}

	if len(run.Error) > maxRunErrorLen {
		run.Error = run.Error[:maxRunErrorLen]
	}
	runs = append(runs, run)

	raw, err := rlp.EncodeToBytes(runs)
	if err != nil {
		return err
	}

	return bucket.Put(txHash, raw)
}

// GetTxRuns loads the contract runs a tx triggered (nil when the tx ran
// no contracts)
func GetTxRuns(txn *bbolt.Tx, txHash []byte) ([]ContractRun, error) {
	raw := txn.Bucket(storage.ReceiptBucketName).Get(txHash)
	if raw == nil {
		return nil, nil
	}

	var runs []ContractRun
	if err := rlp.DecodeBytes(raw, &runs); err != nil {
		return nil, errors.Wrap(err, "broken receipt record")
	}

	return runs, nil
}

// GetTxRuns is the State-level reader for rpc use
func (state *State) GetTxRuns(txHash []byte) ([]ContractRun, error) {
	var runs []ContractRun
	err := state.View(func(txn *bbolt.Tx) error {
		var err error
		runs, err = GetTxRuns(txn, txHash)
		return err
	})

	return runs, err
}
