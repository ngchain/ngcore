package hive

import (
	"sync"

	"github.com/ngchain/ngcore/hive/wasm"
	"github.com/ngchain/ngcore/ngtypes"
)

func TriggerOnNewBlock(block *ngtypes.Block) {
	var wg sync.WaitGroup
	for _, vm := range vmManager.vms {
		wg.Add(1)
		go func(vm *wasm.VM) {
			vm.OnNewBlock(block)
			wg.Done()
		}(vm)
	}
	wg.Wait()
}

func TriggerOnNewTx(tx *ngtypes.Tx) {
	var wg sync.WaitGroup
	for _, vm := range vmManager.vms {
		wg.Add(1)
		go func(vm *wasm.VM) {
			vm.OnNewTx(tx)
			wg.Done()
		}(vm)
	}
	wg.Done()
}

// TODO: add more events
// func TriggerOnNewAccount(account *ngtypes.Account) {}
