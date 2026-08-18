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

// currentAddress is the address whose code is executing right now: the
// top call frame. Host modules (kv/coin/...) dispatch on it, so a
// service callee acts on ITS OWN state
func (vm *VM) currentAddress() ngtypes.Address {
	return vm.frames[len(vm.frames)-1]
}

// callerAddress is the address which invoked the current frame
// (msg.from); the zero address for the outermost frame
func (vm *VM) callerAddress() ngtypes.Address {
	if len(vm.frames) < 2 {
		return ngtypes.Address{}
	}

	return vm.frames[len(vm.frames)-2]
}

// onStack reports whether the address is executing somewhere on the
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
// host wrapper module named by the dependee's address: every exported
// function of the dependency becomes a host function which switches the
// call frame to the dependency's address, runs the export on the
// dependency's own instance, and returns. State effects land on the
// CALLEE's journal slice — this is how shared-ledger contracts (tokens,
// pools) work
func (vm *VM) linkServiceDep(linkName string, depAcc *ngtypes.Contract, depth int) error {
	addr := depAcc.Owner
	depBin, err := LoadContractWasm(depAcc.Source)
	if err != nil {
		return errors.Wrapf(err, "service contract %s does not compile", addr)
	}
	depModule, err := loadModule(depBin, vm.cfg)
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

// serviceCall is the DYNAMIC cross-contract call: it dispatches by a
// RUNTIME address instead of a static import, so one compiled module
// (an AMM pair, a router) works against any target. The target
// runs on ITS OWN state (currentAddress switches to it), sees the
// caller via address.get_caller, reads the passed calldata through
// tx.get_extra, and returns byte payloads through the env buf slots —
// exactly the static-service ABI, resolved at runtime.
//
// Trade-off vs static service/<addr> imports: no compile-time
// dependency, so no immutability guarantee on the target (same as an
// opt-in proxy). Re-entrancy is still blocked and gas still charged.
func (vm *VM) serviceCall(ins *wasman.Instance, addrPtr, argsPtr, argsLen uint32) uint32 {
	vm.charge(gasServiceCall)

	if len(vm.frames) >= maxDepChainDepth {
		panic(errors.Wrapf(ErrDepLimit, "dynamic call chain deeper than %d", maxDepChainDepth))
	}

	target, err := readAddr(ins, addrPtr)
	if err != nil {
		vm.logger.Error(err)
		return 0
	}
	calldata, err := readMem(ins, argsPtr, argsLen)
	if err != nil {
		vm.logger.Error(err)
		return 0
	}
	// copy: the callee runs on a different instance, the caller's
	// memory must not alias the args mid-call
	calldata = append([]byte{}, calldata...)

	// the re-entrancy guard: a contract still executing within this tx
	// cannot be entered again
	if vm.onStack(target) {
		panic(errors.Wrapf(ErrServiceReentry, "contract %s", target))
	}

	acc, err := getContract(vm.txn, target)
	if err != nil || !acc.IsActive() || len(acc.Source) == 0 {
		return 0 // no callable contract at the target
	}

	bin, err := LoadContractWasm(acc.Source)
	if err != nil {
		return 0
	}
	mod, err := wasman.NewModule(vm.cfg, bytes.NewReader(bin))
	if err != nil {
		return 0
	}
	// the target's OWN static deps (if any) link first
	if err := vm.loadContractDeps(mod, len(vm.frames)); err != nil {
		vm.logger.Error(err)
		return 0
	}
	inst, err := vm.linker.Instantiate(mod)
	if err != nil {
		vm.logger.Error(err)
		return 0
	}

	entry, args := resolveDynEntry(mod, calldata)

	// swap the calldata the callee sees, switch the frame, run, restore
	savedArgs := vm.callArgs
	vm.callArgs = args
	vm.frames = append(vm.frames, target)
	_, _, runErr := inst.CallExportedFunc(entry)
	vm.frames = vm.frames[:len(vm.frames)-1]
	vm.callArgs = savedArgs

	if runErr != nil {
		panic(errors.Wrapf(runErr, "dynamic call %s.%s failed", target, entry))
	}

	return 1
}

// resolveDynEntry decodes a calldata's RLP CallData and maps its Method
// to a zero-arg export of the module (init excluded), falling back to
// main — the same rule EntryFor uses for the outer tx
func resolveDynEntry(module *wasman.Module, calldata []byte) (entry string, args []byte) {
	method, a, err := ngtypes.DecodeCallData(calldata)
	if err != nil {
		return VMEntryOnTx, calldata // not a call payload: whole calldata is main's args
	}
	if method == "" || method == VMEntryOnTx || method == VMEntryOnActivate {
		return VMEntryOnTx, a
	}

	sig, ok := exportFuncSig(module, method)
	if !ok || len(sig.InputTypes) != 0 {
		return VMEntryOnTx, a
	}

	return method, a
}

// The dynamic cross-contract call `contract.call` is bound in
// initContractImports; vm.serviceCall (below/above) is its implementation.
