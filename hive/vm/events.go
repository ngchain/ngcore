package vm

import "github.com/ngchain/ngcore/ngtypes"

// call me when applying new block
func (vm *WasmVM) OnNewBlock(block *ngtypes.Block) {
	ext := vm.instance.GetExport("onBlock")
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
func (vm *WasmVM) OnNewTx(tx *ngtypes.Tx) {
	ext := vm.instance.GetExport("onTx")
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
