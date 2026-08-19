package ngstate

import (
	"encoding/hex"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// Receipts are LOCAL, non-consensus data: every node derives them
// deterministically by executing the chain, so they never enter block
// hashes. A tx's receipt is the list of contract runs it triggered.

// Event is one contract-emitted log entry
type Event struct {
	Contract []byte // the address which emitted (the executing frame)
	Topic    string
	Data     []byte
}

// ContractRun records one contract execution triggered by a tx
type ContractRun struct {
	Contract []byte // the address the entry ran on
	Entry    string // main / init
	Ok       bool
	Error    string
	GasUsed  uint64
	Events   []Event
}

// Receipts cross rpc into human hands, so they marshal in the two
// canonical human encodings — addresses as bs58, raw bytes as lowercase
// hex — never Go's default base64 for []byte. (RLP storage is
// untouched: rlp does not consult MarshalJSON.)

func (e Event) MarshalJSON() ([]byte, error) {
	var addr ngtypes.Address
	copy(addr[:], e.Contract)
	return utils.JSON.Marshal(struct {
		Contract string `json:"contract"`
		Topic    string `json:"topic"`
		Data     string `json:"data"`
	}{addr.String(), e.Topic, hex.EncodeToString(e.Data)})
}

func (r ContractRun) MarshalJSON() ([]byte, error) {
	var addr ngtypes.Address
	copy(addr[:], r.Contract)
	return utils.JSON.Marshal(struct {
		Contract string  `json:"contract"`
		Entry    string  `json:"entry"`
		Ok       bool    `json:"ok"`
		Error    string  `json:"error,omitempty"`
		GasUsed  uint64  `json:"gasUsed"`
		Events   []Event `json:"events"`
	}{addr.String(), r.Entry, r.Ok, r.Error, r.GasUsed, r.Events})
}

// emission limits keep the local receipt store abuse-resistant
const (
	maxEventsPerRun  = 128
	maxEventTopicLen = 64
	maxEventDataLen  = 4096
	maxRunErrorLen   = 512

	// receiptRetention is how many recent blocks keep their receipts;
	// older ones prune at checkpoint maintenance (receipts are local
	// convenience data and regenerate on any replay anyway)
	receiptRetention = 16 * ngtypes.BlockCheckRound
)

// txReceipt is the stored record: the settling height tags the runs so
// pruning can age them out without cross-bucket lookups
type txReceipt struct {
	Height uint64
	Runs   []ContractRun
}

// appendContractRun attaches a run record to the tx's receipt
func appendContractRun(txn *bbolt.Tx, txHash []byte, height uint64, run ContractRun) error {
	bucket := txn.Bucket(storage.ReceiptBucketName)

	record := txReceipt{Height: height}
	if raw := bucket.Get(txHash); raw != nil {
		if err := rlp.DecodeBytes(raw, &record); err != nil {
			return errors.Wrap(err, "broken receipt record")
		}
	}

	if len(run.Error) > maxRunErrorLen {
		run.Error = run.Error[:maxRunErrorLen]
	}
	record.Height = height
	record.Runs = append(record.Runs, run)

	raw, err := rlp.EncodeToBytes(&record)
	if err != nil {
		return err
	}

	return bucket.Put(txHash, raw)
}

// PruneReceiptsTxn ages out receipts settled deeper than the retention
// window; called at checkpoint maintenance so the bucket stays bounded
func PruneReceiptsTxn(txn *bbolt.Tx, tipHeight uint64) error {
	if tipHeight <= receiptRetention {
		return nil
	}
	floor := tipHeight - receiptRetention

	c := txn.Bucket(storage.ReceiptBucketName).Cursor()
	for k, raw := c.First(); k != nil; k, raw = c.Next() {
		var record txReceipt
		if err := rlp.DecodeBytes(raw, &record); err != nil || record.Height < floor {
			if err := c.Delete(); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetTxRuns loads the contract runs a tx triggered (nil when the tx ran
// no contracts)
func GetTxRuns(txn *bbolt.Tx, txHash []byte) ([]ContractRun, error) {
	raw := txn.Bucket(storage.ReceiptBucketName).Get(txHash)
	if raw == nil {
		return nil, nil
	}

	var record txReceipt
	if err := rlp.DecodeBytes(raw, &record); err != nil {
		return nil, errors.Wrap(err, "broken receipt record")
	}

	return record.Runs, nil
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
