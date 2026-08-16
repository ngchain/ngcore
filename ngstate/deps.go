package ngstate

import (
	"bytes"
	"encoding/binary"

	"github.com/c0mm4nd/rlp"
	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/pkg/errors"
	"strings"

	"github.com/ngchain/ngcore/ngtypes"
)

// Contracts compose like code modules: a contract imports another
// LOCKED contract's exports through the "contract/<deployer bs58
// address>" namespace, e.g.
// (import "contract/9VLK...Gkb" "swap" (func $swap ...)).
//
// The import section of the wat text declares the dependencies
// STATICALLY, so the chain extracts them at lock time and maintains a
// reference count on every dependee: a depended-on contract can be
// neither deactivated nor destroyed until all dependents released it.
// Activation ordering (a dependee activates before its dependents) makes
// the dependency graph a DAG by construction.

// ContractDepPrefix is the wat import namespace for LIBRARY modules:
// the dependency contributes code running on the caller's state
const ContractDepPrefix = "contract/"

// ServiceDepPrefix is the wat import namespace for SERVICE modules:
// calls run on the dependency's own state (tokens, pools, any shared
// ledger), with address.get_caller exposing the invoking contract
const ServiceDepPrefix = "service/"

const (
	// maxDepsPerContract bounds the direct imports of one contract
	maxDepsPerContract = 32
	// maxDepChainDepth bounds the transitive linking depth
	maxDepChainDepth = 8
)

var (
	ErrDepNotActive     = errors.New("dependency contract is not active")
	ErrDepSelf          = errors.New("contract cannot depend on itself")
	ErrDepLimit         = errors.New("too many contract dependencies")
	ErrContractRefdBy   = errors.New("contract is depended on by other contracts")
	ErrDepInvalidImport = errors.New("malformed contract dependency import")
)

// reserved context keys backing the dependency ledger
const (
	contextKeyDeps = "_deps" // rlp []Address (as [][]byte) on the DEPENDENT
	contextKeyRefs = "_refs" // 8-byte LE counter on the DEPENDEE
)

// resolveDepAddr turns a dependency identifier — the deployer's bs58
// address — into the Address
func resolveDepAddr(raw string) (ngtypes.Address, error) {
	addr, err := ngtypes.NewAddressFromBS58(raw)
	if err != nil {
		return ngtypes.Address{}, errors.Wrapf(ErrDepInvalidImport, "bad dependency address %q: %v", raw, err)
	}

	return addr, nil
}

// extractContractDeps compiles the contract text and returns its
// declared dependency addresses (for the lock-time bookkeeping)
func extractContractDeps(contractText []byte) ([]ngtypes.Address, error) {
	if len(contractText) == 0 {
		return nil, nil
	}

	bin, err := LoadContractWasm(contractText)
	if err != nil {
		return nil, err
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		return nil, err
	}

	deps, err := parseContractDeps(module)
	if err != nil {
		return nil, err
	}

	return resolveDepAddrs(deps)
}

// contractDep is one declared dependency: a library (code on the
// caller's state) or a service (code on its own state). Raw is the
// deployer address as written in the import
type contractDep struct {
	Raw     string
	Service bool
}

// parseContractDeps extracts the declared contract dependencies from a
// compiled module's import section
func parseContractDeps(module *wasman.Module) ([]contractDep, error) {
	seen := make(map[string]bool)
	deps := make([]contractDep, 0)

	for _, imp := range module.ImportSection {
		var addrStr string
		var service bool
		switch {
		case strings.HasPrefix(imp.Module, ContractDepPrefix):
			addrStr = imp.Module[len(ContractDepPrefix):]
		case strings.HasPrefix(imp.Module, ServiceDepPrefix):
			addrStr = imp.Module[len(ServiceDepPrefix):]
			service = true
		default:
			continue
		}

		if !seen[imp.Module] {
			seen[imp.Module] = true
			deps = append(deps, contractDep{Raw: addrStr, Service: service})
		}
	}

	if len(deps) > maxDepsPerContract {
		return nil, errors.Wrapf(ErrDepLimit, "%d imports exceed the cap %d", len(deps), maxDepsPerContract)
	}

	return deps, nil
}

// resolveDepAddrs flattens the dependency set into the unique
// addresses the reference ledger tracks
func resolveDepAddrs(deps []contractDep) ([]ngtypes.Address, error) {
	seen := make(map[ngtypes.Address]bool)
	addrs := make([]ngtypes.Address, 0, len(deps))
	for _, dep := range deps {
		addr, err := resolveDepAddr(dep.Raw)
		if err != nil {
			return nil, err
		}
		if !seen[addr] {
			seen[addr] = true
			addrs = append(addrs, addr)
		}
	}
	return addrs, nil
}

// getContractDeps reads the recorded dependency list of the contract
func getContractDeps(acc *ngtypes.Contract) ([]ngtypes.Address, error) {
	raw := acc.Context.Get(contextKeyDeps)
	if len(raw) == 0 {
		return nil, nil
	}

	var rawDeps [][]byte
	if err := rlp.DecodeBytes(raw, &rawDeps); err != nil {
		return nil, err
	}

	deps := make([]ngtypes.Address, len(rawDeps))
	for i := range rawDeps {
		if len(rawDeps[i]) != ngtypes.AddressSize {
			return nil, errors.Wrapf(ErrDepInvalidImport, "corrupt dep entry %d", i)
		}
		copy(deps[i][:], rawDeps[i])
	}

	return deps, nil
}

func setContractDeps(acc *ngtypes.Contract, deps []ngtypes.Address) error {
	if len(deps) == 0 {
		acc.Context.Del(contextKeyDeps)
		return nil
	}

	rawDeps := make([][]byte, len(deps))
	for i := range deps {
		rawDeps[i] = deps[i].Bytes()
	}

	raw, err := rlp.EncodeToBytes(rawDeps)
	if err != nil {
		return err
	}
	acc.Context.Set(contextKeyDeps, raw)

	return nil
}

// getRefCount reads how many active contracts depend on the contract
func getRefCount(acc *ngtypes.Contract) uint64 {
	raw := acc.Context.Get(contextKeyRefs)
	if len(raw) != 8 {
		return 0
	}

	return binary.LittleEndian.Uint64(raw)
}

func setRefCount(acc *ngtypes.Contract, refs uint64) {
	if refs == 0 {
		acc.Context.Del(contextKeyRefs)
		return
	}

	raw := make([]byte, 8)
	binary.LittleEndian.PutUint64(raw, refs)
	acc.Context.Set(contextKeyRefs, raw)
}
