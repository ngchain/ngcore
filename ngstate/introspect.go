package ngstate

import (
	"bytes"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// ContractExport describes one exported function of a contract module.
type ContractExport struct {
	Name    string
	Params  int
	Results int
	// Callable reports whether a transact tx can dispatch to this export:
	// a zero-argument function export, excluding the reserved init entry
	Callable bool
}

// ContractExports lists the exported functions of a compiled contract
// module — the "ABI" an explorer or wallet shows as callable methods.
func ContractExports(source []byte) ([]ContractExport, error) {
	if len(source) == 0 {
		return nil, nil
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(source))
	if err != nil {
		return nil, err
	}

	var out []ContractExport
	for name, exp := range module.ExportSection {
		if exp.Desc.Kind != 0x00 { // functions only
			continue
		}
		e := ContractExport{Name: name}
		if sig, ok := exportFuncSig(module, name); ok && sig != nil {
			e.Params = len(sig.InputTypes)
			e.Results = len(sig.ReturnTypes)
			e.Callable = len(sig.InputTypes) == 0 && name != VMEntryOnActivate && name != VMEntryOnUpgrade
		}
		out = append(out, e)
	}

	return out, nil
}

// contractHasExport reports whether the compiled contract module statically
// exports a zero-argument function named name (used to check for the UUPS
// `upgrade` hook without running the contract). A malformed source has no
// exports.
func contractHasExport(source []byte, name string) bool {
	if len(source) == 0 {
		return false
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(source))
	if err != nil {
		return false
	}

	sig, ok := exportFuncSig(module, name)
	return ok && sig != nil && len(sig.InputTypes) == 0
}
