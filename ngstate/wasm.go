package ngstate

import (
	"bytes"
	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"strconv"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"

	logging "github.com/ipfs/go-log/v2"
)

// VM is a vm based on wasmtime, which acts as a sandbox env to exec native func
// TODO: Update me after experiment WASI tests
type VM struct {
	sync.RWMutex

	self *ngtypes.Account
	txn  *badger.Txn

	linker *wasman.Linker
	module *wasman.Module

	logger *logging.ZapEventLogger
}

// NewVM creates a new Wasm
// call me when a assign or append tx
func NewVM(txn *badger.Txn, account *ngtypes.Account) (*VM, error) {
	module, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewBuffer(account.Contract))
	if err != nil {
		return nil, err
	}

	linker := wasman.NewLinker(config.LinkerConfig{}) // TODO: add external modules

	return &VM{
		RWMutex: sync.RWMutex{},
		self:    account,
		txn:     txn,
		linker:  linker,
		module:  module,
		logger:  logging.Logger("vm" + strconv.FormatUint(account.Num, 10)),
	}, nil
}

// MaxLen is the maximum length of one module
const MaxLen = 1 << 32 // 2GB

// Instantiate will generate a runable instance from thr module
// before Instantiate, the caller should run Init
func (vm *VM) Instantiate() (*wasman.Instance, error) {
	instance, err := vm.linker.Instantiate(vm.module)
	if err != nil {
		return nil, err
	}

	return instance, nil
}
