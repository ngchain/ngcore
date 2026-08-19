package ngstate

import (
	"bytes"
	"crypto/sha256"
	"sync"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wasm"
	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

const (
	// VMEntryOnTx is the contract export called when an active contract
	// account receives a transact tx
	VMEntryOnTx = "main"
	// VMEntryOnActivate is the optional contract export called once when the
	// account gets activated (contract deployment finished)
	VMEntryOnActivate = "init"

	// vmMaxToll bounds the wasm instruction count per contract call, so a
	// malicious contract cannot stall the chain. Every node uses the same
	// bound to keep execution deterministic
	vmMaxToll = 1 << 24
	// vmCallDepth bounds the wasm call stack depth per contract call
	vmCallDepth uint64 = 512
	// vmMaxMemPages caps a contract instance's linear memory at 64 pages
	// (4 MiB). Toll bounds INSTRUCTIONS, but memory.grow allocates 64 KiB
	// of host memory for ~1 toll — without this consensus cap a contract
	// could grow to wasm32's 4 GiB and exhaust the node
	vmMaxMemPages uint32 = 64

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
	self      *ngtypes.Contract
	txn       *bbolt.Tx
	blockTime uint64 // the enclosing block's timestamp

	journal *vmJournal

	cfg    config.ModuleConfig
	linker *wasman.Linker
	module *wasman.Module

	// frames is the call stack of executing addresses; frames[0] is the
	// contract this vm was built for, service calls push their callee
	frames []ngtypes.Address

	// callArgs is what tx.get_extra serves: the Args of the decoded
	// CallData once EntryFor resolved the method (the whole extra when
	// the call falls back to the default entry)
	callArgs []byte

	// events accumulate during the run and only survive a SUCCESSFUL
	// call (mirroring the journal semantics)
	events []Event

	// tollPreburn is the budget pre-charged by LimitToll (the block
	// gas cap); subtracted when reporting this run's own consumption
	tollPreburn uint64

	// bufs are the cross-frame transfer slots: byte payloads (256-bit
	// amounts, strings) crossing service boundaries go through them,
	// since instances do not share linear memory
	bufs [vmBufSlots][]byte

	logger *logging.ZapEventLogger
}

// wasmMagic is the 4-byte preamble of every WebAssembly binary module
var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

// moduleCache memoizes DECODED + VALIDATED wasm templates by code hash, so
// a contract is parsed and statically validated once instead of on every
// call. The cached template is treated as immutable: callers take a
// shallow copy and bind their OWN per-call ModuleConfig (a fresh
// TollStation etc.) before instantiating. This is safe because wasman's
// Instantiate builds fresh, instance-owned functions/index spaces from the
// (read-only) decoded sections and only writes back to the Module pointer
// it is given — which is the per-call copy, never the shared template.
var moduleCache sync.Map // [32]byte -> *wasman.Module

// templateFor returns the cached decoded+validated template for source,
// building (and caching) it on first sight. A malformed binary returns an
// error and is not cached, so it is re-checked every time (and stays out
// of the hot path).
func templateFor(source []byte) (*wasman.Module, error) {
	key := sha256.Sum256(source)
	if t, ok := moduleCache.Load(key); ok {
		return t.(*wasman.Module), nil
	}
	// validate under the plainest config; the config a template carries is
	// irrelevant since every caller overrides it via loadModule
	m, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	t, _ := moduleCache.LoadOrStore(key, m)
	return t.(*wasman.Module), nil
}

// loadModule returns a per-call Module for source bound to cfg, reusing the
// cached decoded template (no re-parse, no re-validate). The returned value
// is a shallow copy the caller may instantiate; the shared template is not
// mutated.
func loadModule(source []byte, cfg config.ModuleConfig) (*wasman.Module, error) {
	t, err := templateFor(source)
	if err != nil {
		return nil, err
	}
	m := *t              // shallow copy: shares the immutable decoded sections
	m.ModuleConfig = cfg // bind this call's config (fresh toll station, ...)
	return &m, nil
}

// LoadContractWasm validates the on-chain contract bytecode: it must be
// a well-formed WebAssembly binary the vm can load. Contracts are
// authored in any language that targets wasm (Rust, AssemblyScript,
// TinyGo, ...) and the compiled module is what lives on chain.
//
// The source is UNTRUSTED (a malicious commit chooses it) and reaches
// here inside block validation, so a loader panic on adversarial input
// must degrade to an error — never crash the node
func LoadContractWasm(source []byte) (bin []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			bin = nil
			err = errors.Errorf("invalid wasm contract: %v", r)
		}
	}()

	if len(source) < 4 || !bytes.Equal(source[:4], wasmMagic) {
		return nil, errors.New("contract is not a wasm binary (missing \\0asm magic)")
	}

	// a full parse+validate on first sight (cached thereafter): the module
	// must load under the plainest config, so a malformed binary is
	// rejected before execution
	if _, err := templateFor(source); err != nil {
		return nil, errors.Wrap(err, "invalid wasm contract")
	}

	return source, nil
}

// NewVM compiles the address's contract text, binds the built-in host
// modules and links the declared contract dependencies (DAG order, one
// shared gas budget). The tx is the calling tx which triggers this
// execution; blockTime is the enclosing block's timestamp
func NewVM(txn *bbolt.Tx, account *ngtypes.Contract, tx *ngtypes.FullTx, blockTime uint64) (*VM, error) {
	bin, err := LoadContractWasm(account.Source)
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
		// hard host-side cap on linear memory (declared AND grown), so a
		// contract cannot allocate its way past the toll model
		MaxMemoryPages: vmMaxMemPages,
		// wasman's inline-metered JIT: toll counts and results match the
		// interpreter exactly (per-step gas is byte-identical with the JIT on
		// or off, and across architectures), so it is consensus-safe.
		// Requires wasman >= v1.7.1, which fixes host-call dispatch for
		// contracts that share a linker with their service dependencies;
		// v1.7.2 additionally hardens table-element bindings and restores
		// the fast wide-integer dispatch.
		EnableJIT: true,
	}

	module, err := loadModule(bin, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the compiled contract")
	}

	vm := &VM{
		caller:    tx,
		self:      account,
		txn:       txn,
		blockTime: blockTime,
		callArgs:  tx.Extra,
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

// loadContractDeps recursively links the contract dependencies imported
// by the dependee's bs58 ADDRESS. Every dependency is a service: its
// exports run on the dependency's OWN state through a host wrapper module
// named by the address (see linkServiceDep). Dependencies instantiate
// before their dependents (wasman links against instantiated modules
// only), so the whole set loads in DAG order.
func (vm *VM) loadContractDeps(module *wasman.Module, depth int) error {
	deps, err := parseContractDeps(module)
	if err != nil {
		return err
	}
	if len(deps) > 0 && depth >= maxDepChainDepth {
		return errors.Wrapf(ErrDepLimit, "dependency chain deeper than %d", maxDepChainDepth)
	}

	for _, raw := range deps {
		name := raw // the import module name IS the dependee's bs58 address
		if _, linked := vm.linker.Modules[name]; linked {
			continue
		}

		addr, err := resolveDepAddr(raw)
		if err != nil {
			return err
		}
		if addr.Equals(vm.self.Owner) {
			return ErrDepSelf
		}

		depAcc, err := getContract(vm.txn, addr)
		if err != nil {
			return errors.Wrapf(err, "unknown dependency contract %s", addr)
		}
		if !depAcc.IsActive() || len(depAcc.Source) == 0 {
			return errors.Wrapf(ErrDepNotActive, "contract %s", addr)
		}

		// every dependency runs on its OWN state, exposed as a host
		// wrapper module named by the address
		if err := vm.linkServiceDep(name, depAcc, depth); err != nil {
			return err
		}
	}

	return nil
}

// LimitToll shrinks this vm's toll budget below the per-call default,
// implementing the block-level gas cap: the station simply starts
// pre-charged by the difference
func (vm *VM) LimitToll(budget uint64) {
	if budget >= vmMaxToll {
		return
	}

	// pre-burn the part of the default budget the block cannot afford
	vm.tollPreburn = vmMaxToll - budget
	_ = vm.cfg.TollStation.AddToll(vm.tollPreburn)
}

// GasUsed reports the toll THIS run consumed, the pre-burned block-cap
// share excluded
func (vm *VM) GasUsed() uint64 {
	total := vm.cfg.TollStation.GetToll()
	if total <= vm.tollPreburn {
		return 0
	}

	return total - vm.tollPreburn
}

// charge burns extra toll for a host operation; exceeding the budget
// panics, which Recover turns into a deterministic aborted call
func (vm *VM) charge(cost uint64) {
	if err := vm.cfg.TollStation.AddToll(cost); err != nil {
		panic(errors.Wrap(err, "gas budget exceeded by a host operation"))
	}
}

// EntryFor resolves the entry to run. For the default transact entry the
// tx extra is decoded as an RLP CallData{Method, Args}: a non-empty
// Method naming a zero-arg export (the reserved init entry excluded)
// runs that export with Args; an empty/"main" method, an absent export,
// or an extra that is not a CallData all fall back to the default entry,
// which sees the whole extra as its args
func (vm *VM) EntryFor(defaultEntry string) string {
	if defaultEntry != VMEntryOnTx || len(vm.caller.Extra) == 0 {
		return defaultEntry
	}

	method, args, err := ngtypes.DecodeCallData(vm.caller.Extra)
	if err != nil {
		return defaultEntry // not a call payload: the whole extra is main's args
	}

	// a decoded payload always feeds its Args to the entry that runs
	vm.callArgs = args

	if method == "" || method == VMEntryOnTx || method == VMEntryOnActivate {
		return defaultEntry // default entry (init is not tx-callable)
	}
	if sig, ok := exportFuncSig(vm.module, method); !ok || len(sig.InputTypes) != 0 {
		return defaultEntry // no such callable export: fall back to main with the args
	}

	return method
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
