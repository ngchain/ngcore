package wasm

import "github.com/ngchain/ngcore/ngtypes"

// call me when applying new block
func (vm *VM) OnNewBlock(block *ngtypes.Block) {
	ext := vm.instance.GetExport("on_block")
	if ext == nil {
		return
	}

	f := ext.Func()
	if f == nil {
		return
	}

	ok, err := f.Call(block)
	if err != nil {
		log.Error(err)
	}

	if !(ok.(bool)) {
		log.Error("fail on calling onBlock")
	}
}

// call me when applying new tx
func (vm *VM) OnNewTx(tx *ngtypes.Tx) {
	ext := vm.instance.GetExport("on_tx")
	if ext == nil {
		return
	}

	f := ext.Func()
	if f == nil {
		return
	}

	ok, err := f.Call(tx)
	if err != nil {
		log.Error(err)
	}

	if !(ok.(bool)) {
		log.Error("fail on calling onTx")
	}
}
