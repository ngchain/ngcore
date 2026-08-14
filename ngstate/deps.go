package ngstate

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/c0mm4nd/rlp"
	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// Contracts compose like code modules: a contract imports another
// LOCKED contract's exports through the "contract/<num>" namespace,
// e.g. (import "contract/42" "swap" (func $swap ...)).
//
// The import section of the wat text declares the dependencies
// STATICALLY, so the chain extracts them at lock time and maintains a
// reference count on every dependee: a depended-on contract can be
// neither unlocked nor destroyed until all dependents released it.
// Lock ordering (a dependee must be locked before its dependents) makes
// the dependency graph a DAG by construction.

// ContractDepPrefix is the wat import namespace for contract modules
const ContractDepPrefix = "contract/"

const (
	// maxDepsPerContract bounds the direct imports of one contract
	maxDepsPerContract = 32
	// maxDepChainDepth bounds the transitive linking depth
	maxDepChainDepth = 8
)

var (
	ErrDepNotActive     = errors.New("dependency contract is not locked")
	ErrDepSelf          = errors.New("contract cannot depend on itself")
	ErrDepLimit         = errors.New("too many contract dependencies")
	ErrAccountRefdBy    = errors.New("account is depended on by other contracts")
	ErrDepInvalidImport = errors.New("malformed contract dependency import")
)

// reserved context keys backing the dependency ledger
const (
	contextKeyDeps = "_deps" // rlp []uint64 on the DEPENDENT
	contextKeyRefs = "_refs" // 8-byte LE counter on the DEPENDEE
)

// extractContractDeps compiles the contract text and returns its
// declared dependencies (for the lock-time bookkeeping)
func extractContractDeps(contractText []byte) ([]uint64, error) {
	if len(contractText) == 0 {
		return nil, nil
	}

	bin, err := CompileContract(contractText)
	if err != nil {
		return nil, err
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		return nil, err
	}

	return parseContractDeps(module)
}

// parseContractDeps extracts the declared contract dependencies from a
// compiled module's import section
func parseContractDeps(module *wasman.Module) ([]uint64, error) {
	seen := make(map[uint64]bool)
	deps := make([]uint64, 0)

	for _, imp := range module.ImportSection {
		if !strings.HasPrefix(imp.Module, ContractDepPrefix) {
			continue
		}

		num, err := strconv.ParseUint(imp.Module[len(ContractDepPrefix):], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(ErrDepInvalidImport, "bad import namespace %q", imp.Module)
		}

		if !seen[num] {
			seen[num] = true
			deps = append(deps, num)
		}
	}

	if len(deps) > maxDepsPerContract {
		return nil, errors.Wrapf(ErrDepLimit, "%d imports exceed the cap %d", len(deps), maxDepsPerContract)
	}

	return deps, nil
}

// getContractDeps reads the recorded dependency list of the account
func getContractDeps(acc *ngtypes.Account) ([]uint64, error) {
	raw := acc.Context.Get(contextKeyDeps)
	if len(raw) == 0 {
		return nil, nil
	}

	var deps []uint64
	if err := rlp.DecodeBytes(raw, &deps); err != nil {
		return nil, err
	}

	return deps, nil
}

func setContractDeps(acc *ngtypes.Account, deps []uint64) error {
	if len(deps) == 0 {
		acc.Context.Del(contextKeyDeps)
		return nil
	}

	raw, err := rlp.EncodeToBytes(deps)
	if err != nil {
		return err
	}
	acc.Context.Set(contextKeyDeps, raw)

	return nil
}

// getRefCount reads how many locked contracts depend on the account
func getRefCount(acc *ngtypes.Account) uint64 {
	raw := acc.Context.Get(contextKeyRefs)
	if len(raw) != 8 {
		return 0
	}

	return binary.LittleEndian.Uint64(raw)
}

func setRefCount(acc *ngtypes.Account, refs uint64) {
	if refs == 0 {
		acc.Context.Del(contextKeyRefs)
		return
	}

	raw := make([]byte, 8)
	binary.LittleEndian.PutUint64(raw, refs)
	acc.Context.Set(contextKeyRefs, raw)
}
