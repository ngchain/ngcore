package hive

import (
	"sync"

	"github.com/ngchain/ngcore/hive/wasm"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

func triggerOnBlock(block *ngtypes.Block) {
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

func triggerOnTx(tx *ngtypes.Tx) {
	for _, addr := range tx.Participants {
		
	}
}

// TODO: add more events
// func TriggerOnNewAccount(account *ngtypes.Account) {}
