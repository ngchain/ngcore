package ngstate

import (
	"bytes"
	"reflect"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/types"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

var (
	ErrServiceReentry   = errors.New("re-entering a contract which is still executing")
	ErrServiceBadExport = errors.New("service export is not linkable")
)

// currentAccount is the account whose code is executing right now: the
// top call frame. Host modules (kv/coin/...) dispatch on it, so a
// service callee acts on ITS OWN state
func (vm *VM) currentAccount() ngtypes.Address {
	return vm.frames[len(vm.frames)-1]
}

// callerAccount is the account which invoked the current frame
// (msg.sender); the zero address for the outermost frame
func (vm *VM) callerAccount() ngtypes.Address {
	if len(vm.frames) < 2 {
		return ngtypes.Address{}
	}

	return vm.frames[len(vm.frames)-2]
}

// onStack reports whether the account is executing somewhere on the
// current call path
func (vm *VM) onStack(addr ngtypes.Address) bool {
	for _, f := range vm.frames {
		if f == addr {
			return true
		}
	}

	return false
}

// linkServiceDep instantiates the dependency contract and synthesizes a
// host wrapper module under service/<num>: every exported function of
// the dependency becomes a host function which switches the call frame
// to the dependency's account, runs the export on the dependency's own
// instance, and returns. State effects land on the CALLEE's journal
// slice — this is how shared-ledger contracts (tokens, pools) work
func (vm *VM) linkServiceDep(linkName string, depAcc *ngtypes.Account, depth int) error {
	addr := depAcc.Owner
	depBin, err := CompileContract(depAcc.Contract)
	if err != nil {
		return errors.Wrapf(err, "service contract %s does not compile", addr)
	}
	depModule, err := wasman.NewModule(vm.cfg, bytes.NewReader(depBin))
	if err != nil {
		return errors.Wrapf(err, "failed to load service contract %s", addr)
	}

	// the service's own dependencies resolve first (DAG order)
	if err := vm.loadContractDeps(depModule, depth+1); err != nil {
		return err
	}

	ins, err := vm.linker.Instantiate(depModule)
	if err != nil {
		return errors.Wrapf(err, "failed to instantiate service contract %s", addr)
	}

	for exportName := range depModule.ExportSection {
		sig, ok := exportFuncSig(depModule, exportName)
		if !ok {
			continue // non-function exports are not linkable as services
		}

		wrapper := vm.makeServiceWrapper(addr, ins, exportName, sig)
		if err := vm.linker.DefineAdvancedFunc(linkName, exportName, func(_ *wasman.Instance) interface{} {
			return wrapper
		}); err != nil {
			return errors.Wrapf(ErrServiceBadExport, "%s.%s: %v", linkName, exportName, err)
		}
	}

	return nil
}

// exportFuncSig resolves the wasm type of a module's exported function
func exportFuncSig(module *wasman.Module, exportName string) (*types.FuncType, bool) {
	export, ok := module.ExportSection[exportName]
	if !ok || export.Desc.Kind != 0x00 {
		return nil, false
	}

	numImportedFuncs := uint32(0)
	for _, imp := range module.ImportSection {
		if imp.Desc.Kind == 0x00 {
			numImportedFuncs++
		}
	}

	if export.Desc.Index < numImportedFuncs {
		return nil, false // re-exported import: not supported as a service entry
	}

	typeIdx := module.FunctionSection[export.Desc.Index-numImportedFuncs]
	if int(typeIdx) >= len(module.TypeSection) {
		return nil, false
	}

	return module.TypeSection[typeIdx], true
}

// makeServiceWrapper fabricates a Go func matching the export's wasm
// signature which pushes the callee frame, forwards into the callee's
// instance and pops. Failures (incl. the reentry guard) panic: the
// module's Recover turns that into an aborted call with a dropped journal
func (vm *VM) makeServiceWrapper(addr ngtypes.Address, ins *wasman.Instance, exportName string, sig *types.FuncType) interface{} {
	in := make([]reflect.Type, len(sig.InputTypes))
	for i, t := range sig.InputTypes {
		in[i] = goTypeOf(t)
	}
	out := make([]reflect.Type, len(sig.ReturnTypes))
	for i, t := range sig.ReturnTypes {
		out[i] = goTypeOf(t)
	}

	fnType := reflect.FuncOf(in, out, false)

	impl := func(args []reflect.Value) []reflect.Value {
		// the reentry guard: a contract still executing within this tx
		// cannot be entered again
		if vm.onStack(addr) {
			panic(errors.Wrapf(ErrServiceReentry, "contract %s", addr))
		}

		vm.charge(gasServiceCall)

		raw := make([]uint64, len(args))
		for i, a := range args {
			raw[i] = toRaw(a)
		}

		vm.frames = append(vm.frames, addr)
		rets, _, err := ins.CallExportedFunc(exportName, raw...)
		vm.frames = vm.frames[:len(vm.frames)-1]

		if err != nil {
			panic(errors.Wrapf(err, "service call %s.%s failed", addr, exportName))
		}
		if len(rets) != len(out) {
			panic(errors.Wrapf(ErrServiceBadExport, "%s.%s returned %d values", addr, exportName, len(rets)))
		}

		results := make([]reflect.Value, len(out))
		for i, t := range out {
			results[i] = fromRaw(rets[i], t)
		}

		return results
	}

	return reflect.MakeFunc(fnType, impl).Interface()
}

func goTypeOf(t types.ValueType) reflect.Type {
	switch t {
	case types.ValueTypeI32:
		return reflect.TypeOf(uint32(0))
	case types.ValueTypeI64:
		return reflect.TypeOf(uint64(0))
	case types.ValueTypeF32:
		return reflect.TypeOf(float32(0))
	default:
		return reflect.TypeOf(float64(0))
	}
}

func toRaw(v reflect.Value) uint64 {
	switch v.Kind() {
	case reflect.Uint32, reflect.Uint64:
		return v.Uint()
	default:
		panic(errors.Wrap(ErrServiceBadExport, "float service params are disabled"))
	}
}

func fromRaw(raw uint64, t reflect.Type) reflect.Value {
	switch t.Kind() {
	case reflect.Uint32:
		return reflect.ValueOf(uint32(raw))
	case reflect.Uint64:
		return reflect.ValueOf(raw)
	default:
		panic(errors.Wrap(ErrServiceBadExport, "float service returns are disabled"))
	}
}
