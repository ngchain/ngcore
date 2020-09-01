package wasm_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"

	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/storage"

	"github.com/ngchain/ngcore/hive/wasm"
)

func TestNewWasmVM(t *testing.T) {
	// requires the consensus here
	db := storage.InitMemStorage()
	ngpool.Init(db)
	ngblocks.Init(db)
	ngchain.Init(db)
	ngstate.InitStateFromGenesis(db)
	ngp2p.InitLocalNode(52520)

	f, _ := os.Open("test/contract.wasm")
	raw, err := ioutil.ReadAll(f) // TODO: implement a mvp
	if err != nil {
		panic(err)
	}

	contract, err := wasm.NewVM(500, raw)
	if err != nil {
		panic(err)
	}

	err = contract.InitBuiltInImports()
	if err != nil {
		panic(err)
	}

	err = contract.Instantiate()
	if err != nil {
		panic(err)
	}

	contract.Start() // will receive error but main thread wont panic
}
