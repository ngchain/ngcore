package hive

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"strconv"
	"sync"

	"github.com/bytecodealliance/wasmtime-go"
	logging "github.com/ipfs/go-log/v2"
)

var engine = wasmtime.NewEngine()

// VM is a vm based on wasmtime, which acts as a sandbox env to exec native func
// TODO: Update me after experiment WASI tests
type VM struct {
	sync.RWMutex

	self *ngtypes.Account
	txn  *badger.Txn

	linker   *wasmtime.Linker
	store    *wasmtime.Store
	module   *wasmtime.Module
	instance *wasmtime.Instance

	logger *logging.ZapEventLogger
}

// NewVM creates a new Wasm
func NewVM(txn *badger.Txn, account *ngtypes.Account) (*VM, error) {
	store := wasmtime.NewStore(engine)
	module, err := wasmtime.NewModule(engine, account.Contract)
	if err != nil {
		return nil, err
	}

	linker := wasmtime.NewLinker(store)

	return &VM{
		RWMutex:  sync.RWMutex{},
		self:     account,
		txn:      txn,
		linker:   linker,
		store:    store,
		module:   module,
		instance: nil,
		logger:   logging.Logger("vm" + strconv.FormatUint(account.Num, 10)),
	}, nil
}

func (vm *VM) GetStore() *wasmtime.Store {
	vm.RLock()
	store := vm.store
	vm.RUnlock()

	return store
}

func (vm *VM) GetModule() *wasmtime.Module {
	vm.RLock()
	module := vm.module
	vm.RUnlock()

	return module
}

const MaxLen = 1 << 32 // 2GB

func (vm *VM) Instantiate() error {
	instance, err := vm.linker.Instantiate(vm.module)
	if err != nil {
		return err
	}

	vm.Lock()

	vm.instance = instance

	// init context
	mem := vm.instance.GetExport("memory").Memory()
	length := mem.DataSize()
	if length >= MaxLen {
		length = MaxLen // avoid panic
	}

	vm.Unlock()
	return nil
}
