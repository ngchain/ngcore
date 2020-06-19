package vm

import "github.com/bytecodealliance/wasmtime-go"

func getWASICfg() *wasmtime.WasiConfig {
	wasiCfg := wasmtime.NewWasiConfig()
	wasiCfg.SetArgv([]string{""}) // some extensions like block hash etc, first arg points to program itself. The args act as an input.

	return wasiCfg
}
