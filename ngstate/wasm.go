package ngstate

import (
	"bytes"

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

	// vmBufSlots / vmBufMaxLen bound the cross-frame transfer slots
	vmBufSlots  = 8
	vmBufMaxLen = 4096
)

// host operations charge extra toll on top of the flat per-instruction
// price: state writes are orders of magnitude more expensive than
// arithmetic and must be priced accordingly
const (
	gasKVSetBase    = 1000
	gasKVSetPerByte = 10
	gasKVDel        = 500
	gasKVRead       = 100
	gasCoinTransfer = 2000
	gasEventBase    = 500
	gasEventPerByte = 5
	gasServiceCall  = 2000
)

// VM is the sandbox env for exec a contract, based on wasman.
// A fresh VM is built for every single contract call: nothing inside it
// survives across txs, which keeps the execution deterministic
type VM struct {
	caller    *ngtypes.FullTx
	self      *ngtypes.Account
	txn       *bbolt.Tx
	blockTime uint64 // the enclosing block's timestamp

	journal *vmJournal

	cfg    config.ModuleConfig
	linker *wasman.Linker
	module *wasman.Module

	// frames is the call stack of executing addresses; frames[0] is the
	// contract this vm was built for, service calls push their callee
	frames []ngtypes.Address

	// events accumulate during the run and only survive a SUCCESSFUL
	// call (mirroring the journal semantics)
	events []Event

	// bufs are the cross-frame transfer slots: byte payloads (256-bit
	// amounts, strings) crossing service boundaries go through them,
	// since instances do not share linear memory
	bufs [vmBufSlots][]byte

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

// NewVM compiles the account's contract text, binds the built-in host
// modules and links the declared contract dependencies (DAG order, one
// shared gas budget). The tx is the calling tx which triggers this
// execution; blockTime is the enclosing block's timestamp
func NewVM(txn *bbolt.Tx, account *ngtypes.Account, tx *ngtypes.FullTx, blockTime uint64) (*VM, error) {
	bin, err := CompileContract(account.Contract)
	if err != nil {
		return nil, err
	}

	callDepth := vmCallDepth
	cfg := config.ModuleConfig{
		DisableFloatPoint: true, // floats are not deterministic across platforms
		Recover:           true, // a contract panic must never kill the node
		CallDepthLimit:    &callDepth,
		// the deterministic wide-integer host modules ("u128"/"u256"
		// import namespaces): 256-bit token amounts without float math
		EnableWideInt: true,
		// ONE toll station across the whole link set: dependency code
		// burns the caller's gas
		TollStation: tollstation.NewSimpleTollStation(vmMaxToll),
	}

	module, err := wasman.NewModule(cfg, bytes.NewReader(bin))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the compiled contract")
	}

	vm := &VM{
		caller:    tx,
		self:      account,
		txn:       txn,
		blockTime: blockTime,
		journal:   newVMJournal(account),
		frames:    []ngtypes.Address{account.Owner},
		cfg:       cfg,
		linker:    wasman.NewLinker(config.LinkerConfig{}),
		module:    module,
		logger:    logging.Logger("vm-" + account.Owner.String()[:8]),
	}

	err = vm.initBuiltInImports()
	if err != nil {
		return nil, err
	}

	// the host modules are in place: link the contract dependencies
	if err := vm.loadContractDeps(module, 0); err != nil {
		return nil, err
	}

	return vm, nil
}

// loadContractDeps recursively links the contract modules imported
// through the contract/<bs58 addr> namespace. Dependencies instantiate before
// their dependents (wasman links against instantiated modules only), so
// the whole set loads in dependency order.
//
// NOTE the delegate semantics: dependency code runs with THIS vm's host
// modules, so its kv/coin effects act on the CALLING account's state —
// a dependency contributes code, not its own state
func (vm *VM) loadContractDeps(module *wasman.Module, depth int) error {
	deps, err := parseContractDeps(module)
	if err != nil {
		return err
	}
	if len(deps) > 0 && depth >= maxDepChainDepth {
		return errors.Wrapf(ErrDepLimit, "dependency chain deeper than %d", maxDepChainDepth)
	}

	for _, dep := range deps {
		prefix := ContractDepPrefix
		if dep.Service {
			prefix = ServiceDepPrefix
		}
		name := prefix + dep.Raw
		if _, linked := vm.linker.Modules[name]; linked {
			continue
		}

		addr, err := resolveDepAddr(dep.Raw)
		if err != nil {
			return err
		}
		if addr.Equals(vm.self.Owner) {
			return ErrDepSelf
		}

		depAcc, err := getAccount(vm.txn, addr)
		if err != nil {
			return errors.Wrapf(err, "unknown dependency contract %s", addr)
		}
		if !depAcc.IsLocked() || len(depAcc.Contract) == 0 {
			return errors.Wrapf(ErrDepNotActive, "contract %s", addr)
		}

		if dep.Service {
			// service: own-state semantics via a host wrapper module
			if err := vm.linkServiceDep(name, depAcc, depth); err != nil {
				return err
			}
			continue
		}

		// library: the dependency's code links directly and runs on the
		// caller's state
		depBin, err := CompileContract(depAcc.Contract)
		if err != nil {
			return errors.Wrapf(err, "dependency contract %s does not compile", addr)
		}
		depModule, err := wasman.NewModule(vm.cfg, bytes.NewReader(depBin))
		if err != nil {
			return errors.Wrapf(err, "failed to load dependency contract %s", addr)
		}

		// resolve the dependency's own imports first (DAG order)
		if err := vm.loadContractDeps(depModule, depth+1); err != nil {
			return err
		}

		if _, err := vm.linker.Instantiate(depModule); err != nil {
			return errors.Wrapf(err, "failed to instantiate dependency contract %s", addr)
		}

		vm.linker.Define(name, depModule)
	}

	return nil
}

// charge burns extra toll for a host operation; exceeding the budget
// panics, which Recover turns into a deterministic aborted call
func (vm *VM) charge(cost uint64) {
	if err := vm.cfg.TollStation.AddToll(cost); err != nil {
		panic(errors.Wrap(err, "gas budget exceeded by a host operation"))
	}
}

// Events returns what the (successful) run emitted
func (vm *VM) Events() []Event {
	return vm.events
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

// DryRun executes the entry like Run but NEVER flushes the journal:
// the chain state stays untouched, making it safe inside a read-only
// txn. It reports the gas the call burned
func (vm *VM) DryRun(entry string) (gasUsed uint64, err error) {
	ins, err := vm.linker.Instantiate(vm.module)
	if err != nil {
		return vm.cfg.TollStation.GetToll(), errors.Wrap(err, "failed to instantiate the contract")
	}

	_, _, err = ins.CallExportedFunc(entry)

	return vm.cfg.TollStation.GetToll(), err
}

// IsExportMissing tells whether the Run error is just the contract not
// exporting the entry, which is legal for optional entries like init
func IsExportMissing(err error) bool {
	return errors.Is(err, wasm.ErrExportedFuncNotFound)
}
