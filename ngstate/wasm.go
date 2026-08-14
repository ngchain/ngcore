package ngstate

import (
	"bytes"
	"strconv"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wasm"
	"github.com/c0mm4nd/wasman/wat"
	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

const (
	// VMEntryOnTx is the contract export called when a locked contract
	// account receives a transact tx
	VMEntryOnTx = "main"
	// VMEntryOnLock is the optional contract export called once when the
	// account gets locked (contract deployment finished)
	VMEntryOnLock = "init"

	// vmMaxToll bounds the wasm instruction count per contract call, so a
	// malicious contract cannot stall the chain. Every node uses the same
	// bound to keep execution deterministic
	vmMaxToll = 1 << 24
	// vmCallDepth bounds the wasm call stack depth per contract call
	vmCallDepth uint64 = 512
)

// VM is the sandbox env for exec a contract, based on wasman.
// A fresh VM is built for every single contract call: nothing inside it
// survives across txs, which keeps the execution deterministic
type VM struct {
	caller *ngtypes.FullTx
	self   *ngtypes.Account
	txn    *bbolt.Tx

	journal *vmJournal

	linker *wasman.Linker
	module *wasman.Module

	logger *logging.ZapEventLogger
}

// CompileContract translates the on-chain contract text (wat) into its
// binary encoding. The compilation is deterministic, so every node gets
// the same bytes for the same on-chain text
func CompileContract(contract []byte) ([]byte, error) {
	bin, err := wat.Compile(contract)
	if err != nil {
		return nil, errors.Wrap(err, "contract text does not compile")
	}

	return bin, nil
}

// NewVM compiles the account's contract text and binds the built-in host
// modules. The tx is the calling tx which triggers this execution
func NewVM(txn *bbolt.Tx, account *ngtypes.Account, tx *ngtypes.FullTx) (*VM, error) {
	bin, err := CompileContract(account.Contract)
	if err != nil {
		return nil, err
	}

	callDepth := vmCallDepth
	module, err := wasman.NewModule(config.ModuleConfig{
		DisableFloatPoint: true, // floats are not deterministic across platforms
		Recover:           true, // a contract panic must never kill the node
		CallDepthLimit:    &callDepth,
		TollStation:       tollstation.NewSimpleTollStation(vmMaxToll),
	}, bytes.NewReader(bin))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the compiled contract")
	}

	vm := &VM{
		caller:  tx,
		self:    account,
		txn:     txn,
		journal: newVMJournal(account),
		linker:  wasman.NewLinker(config.LinkerConfig{}),
		module:  module,
		logger:  logging.Logger("vm" + strconv.FormatUint(account.Num, 10)),
	}

	err = vm.initBuiltInImports()
	if err != nil {
		return nil, err
	}

	return vm, nil
}

// Run instantiates the module and calls the entry export.
// All the state changes made by the contract go into the journal and are
// flushed into the db txn only when the call fully succeeds; a trap, a
// toll overflow or a missing entry leaves the state untouched
func (vm *VM) Run(entry string) error {
	ins, err := vm.linker.Instantiate(vm.module)
	if err != nil {
		return errors.Wrap(err, "failed to instantiate the contract")
	}

	_, _, err = ins.CallExportedFunc(entry)
	if err != nil {
		return errors.Wrapf(err, "contract call %s failed", entry)
	}

	return vm.journal.flush(vm.txn)
}

// IsExportMissing tells whether the Run error is just the contract not
// exporting the entry, which is legal for optional entries like init
func IsExportMissing(err error) bool {
	return errors.Is(err, wasm.ErrExportedFuncNotFound)
}
