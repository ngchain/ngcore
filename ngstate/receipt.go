package ngstate

import (
	"bytes"
	"encoding/hex"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// EventTopicPrefix namespaces system-emitted events. Contracts may NOT
// emit a topic under it (log.emit rejects them), so a consumer can trust
// that a log with such a topic is genuine node-derived data, not
// contract-forged.
const EventTopicPrefix = "ng."

// EventTopicTransfer is the topic of the log auto-emitted for every native
// value transfer a contract makes through coin.transfer — the "internal
// transaction" surfaced by ng_getLogs with no separate tracing subsystem.
// The emitter (Event.Contract) is the sender; Data is to(32) ‖ value(32,
// the LE money format).
const EventTopicTransfer = EventTopicPrefix + "transfer"

// Receipts are LOCAL, non-consensus data: every node derives them
// deterministically by executing the chain, so they never enter block
// hashes. A tx's receipt is the list of contract runs it triggered.

// Event is one contract-emitted log entry
type Event struct {
	Contract []byte // the address which emitted (the executing frame)
	Topic    string
	Data     []byte
}

// TraceCall is one internal step of a contract execution — a
// cross-contract call or a native value transfer — recorded in execution
// (pre-order) with its Depth, so the flat list reconstructs the call tree.
// Unlike events, the trace is KEPT on a failed run: it shows where the
// call reverted (Ok marks each frame that completed before the abort).
type TraceCall struct {
	Type   string // "call" (contract.call) | "transfer" (coin.transfer)
	Depth  uint32
	From   []byte
	To     []byte
	Method string // the export the call invoked ("" for transfers)
	Value  []byte // 32-byte LE amount (transfers)
	Input  []byte // calldata args (dynamic calls)
	Ok     bool
}

// ContractRun records one contract execution triggered by a tx
type ContractRun struct {
	Contract []byte // the address the entry ran on
	Entry    string // main / init
	Ok       bool
	Error    string
	GasUsed  uint64
	Events   []Event
	Trace    []TraceCall `rlp:"optional"` // trailing-optional: old receipts decode with nil
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

func (c TraceCall) MarshalJSON() ([]byte, error) {
	var from, to ngtypes.Address
	copy(from[:], c.From)
	copy(to[:], c.To)
	return utils.JSON.Marshal(struct {
		Type   string `json:"type"`
		Depth  uint32 `json:"depth"`
		From   string `json:"from"`
		To     string `json:"to"`
		Method string `json:"method,omitempty"`
		Value  string `json:"value,omitempty"`
		Input  string `json:"input,omitempty"`
		Ok     bool   `json:"ok"`
	}{c.Type, c.Depth, from.String(), to.String(), c.Method,
		hex.EncodeToString(c.Value), hex.EncodeToString(c.Input), c.Ok})
}

func (r ContractRun) MarshalJSON() ([]byte, error) {
	var addr ngtypes.Address
	copy(addr[:], r.Contract)
	return utils.JSON.Marshal(struct {
		Contract string      `json:"contract"`
		Entry    string      `json:"entry"`
		Ok       bool        `json:"ok"`
		Error    string      `json:"error,omitempty"`
		GasUsed  uint64      `json:"gasUsed"`
		Events   []Event     `json:"events"`
		Trace    []TraceCall `json:"trace,omitempty"`
	}{addr.String(), r.Entry, r.Ok, r.Error, r.GasUsed, r.Events, r.Trace})
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

// appendContractRun attaches a run record to the tx's receipt (a tx can
// trigger several runs). Re-applying a tx on a reorg branch would double
// its runs, so the reorg unwind clears the reverted heights' receipts
// first (deleteReceiptsAboveTxn) — this stays a plain append.
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

// deleteReceiptsAboveTxn drops every receipt settled above height, used by
// the reorg unwind so re-applying the branch rebuilds receipts fresh
// instead of appending onto the orphaned branch's runs
func deleteReceiptsAboveTxn(txn *bbolt.Tx, height uint64) error {
	c := txn.Bucket(storage.ReceiptBucketName).Cursor()
	for k, raw := c.First(); k != nil; k, raw = c.Next() {
		var record txReceipt
		if err := rlp.DecodeBytes(raw, &record); err != nil {
			continue // leave undecodable entries for PruneReceiptsTxn
		}
		if record.Height > height {
			if err := c.Delete(); err != nil {
				return err
			}
		}
	}
	return nil
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

// ReceiptRetainedFloor is the lowest height whose receipts are still
// available: 0 on an archive node (all kept), else tip - receiptRetention.
// Log/trace reads below it cannot be answered truthfully and are refused
// rather than returning a partial (empty) result.
func (state *State) ReceiptRetainedFloor() (uint64, error) {
	if state.Archive {
		return 0, nil
	}
	var floor uint64
	err := state.View(func(txn *bbolt.Tx) error {
		tip, err := ngblocks.GetLatestHeight(txn.Bucket(storage.BlockBucketName))
		if err != nil {
			return err
		}
		if tip > receiptRetention {
			floor = tip - receiptRetention
		}
		return nil
	})
	return floor, err
}

// LogFilter selects logs by a block-height range and optional emitter and
// topic. ToHeight 0 means up to the tip
type LogFilter struct {
	FromHeight uint64
	ToHeight   uint64
	Address    *ngtypes.Address // nil = any emitter
	Topic      *string          // nil = any topic
}

// Log is one matched event with its on-chain location
type Log struct {
	Height   uint64
	TxHash   []byte
	RunIndex int
	LogIndex int
	Event    Event
}

// maxLogRange caps the block span of one getLogs query so a single call
// cannot scan the whole chain
const maxLogRange = 10000

// GetLogs scans the receipts of the blocks in the filter's height range
// and returns the events matching the emitter/topic. Contract events and
// the auto-emitted native-transfer logs are both surfaced. On a
// non-archive node receipts below the retention window are pruned, so a
// range reaching into pruned history is refused rather than silently
// returning a partial result.
func (state *State) GetLogs(f LogFilter) ([]Log, error) {
	var out []Log
	err := state.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		tip, err := ngblocks.GetLatestHeight(blockBucket)
		if err != nil {
			return err
		}
		if f.ToHeight == 0 || f.ToHeight > tip {
			f.ToHeight = tip
		}
		if f.FromHeight > f.ToHeight {
			return nil
		}
		if f.ToHeight-f.FromHeight >= maxLogRange {
			return errors.Errorf("log range %d..%d spans more than %d blocks", f.FromHeight, f.ToHeight, maxLogRange)
		}
		if !state.Archive && tip > receiptRetention {
			if floor := tip - receiptRetention; f.FromHeight < floor {
				return errors.Errorf("logs before height %d are pruned (node is not in archive mode)", floor)
			}
		}

		for h := f.FromHeight; h <= f.ToHeight; h++ {
			block, err := ngblocks.GetBlockByHeight(blockBucket, h)
			if err != nil {
				return err
			}
			for _, tx := range block.Txs {
				txHash := tx.GetHash()
				runs, err := GetTxRuns(txn, txHash)
				if err != nil {
					return err
				}
				for ri := range runs {
					for li := range runs[ri].Events {
						ev := runs[ri].Events[li]
						if f.Address != nil && !bytes.Equal(ev.Contract, f.Address[:]) {
							continue
						}
						if f.Topic != nil && ev.Topic != *f.Topic {
							continue
						}
						out = append(out, Log{Height: h, TxHash: txHash, RunIndex: ri, LogIndex: li, Event: ev})
					}
				}
			}
		}
		return nil
	})

	return out, err
}
