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
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
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

// ContractDepPrefix is the wat import namespace for LIBRARY modules:
// the dependency contributes code running on the caller's state
const ContractDepPrefix = "contract/"

// ServiceDepPrefix is the wat import namespace for SERVICE modules:
// calls run on the dependency's own state (tokens, pools, any shared
// ledger), with account.get_caller exposing the invoking contract
const ServiceDepPrefix = "service/"

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
	contextKeyName = "_name" // the registered name of the contract
)

// maxContractNameLen bounds a registered contract name
const maxContractNameLen = 32

var (
	ErrNameInvalid = errors.New("invalid contract name")
	ErrNameTaken   = errors.New("contract name is already registered")
	ErrNameUnknown = errors.New("unknown contract name")
)

// validContractName enforces [a-z0-9_-]{1,32}
func validContractName(name string) bool {
	if len(name) == 0 || len(name) > maxContractNameLen {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func contractNameKey(deployer ngtypes.Address, name string) []byte {
	return append(append([]byte{}, deployer[:]...), name...)
}

// getNumByName resolves a (deployer, name) pair to the hosting account
func getNumByName(txn *bbolt.Tx, deployer ngtypes.Address, name string) (uint64, error) {
	raw := txn.Bucket(storage.ContractNameBucketName).Get(contractNameKey(deployer, name))
	if len(raw) != 8 {
		return 0, errors.Wrapf(ErrNameUnknown, "%s.%s", deployer.String(), name)
	}

	return binary.LittleEndian.Uint64(raw), nil
}

func setContractName(txn *bbolt.Tx, deployer ngtypes.Address, name string, num uint64) error {
	raw := make([]byte, 8)
	binary.LittleEndian.PutUint64(raw, num)

	return txn.Bucket(storage.ContractNameBucketName).Put(contractNameKey(deployer, name), raw)
}

func delContractName(txn *bbolt.Tx, deployer ngtypes.Address, name string) error {
	return txn.Bucket(storage.ContractNameBucketName).Delete(contractNameKey(deployer, name))
}

// resolveDepNum turns a dependency identifier into the hosting account
// num. Two forms exist:
//   - "700"                     — the raw account num
//   - "<deployerBS58>.<name>"   — the registered addr.name handle
func resolveDepNum(txn *bbolt.Tx, raw string) (uint64, error) {
	if dot := strings.IndexByte(raw, '.'); dot >= 0 {
		deployer, err := ngtypes.NewAddressFromBS58(raw[:dot])
		if err != nil {
			return 0, errors.Wrapf(ErrDepInvalidImport, "bad deployer address in %q: %v", raw, err)
		}

		name := raw[dot+1:]
		if !validContractName(name) {
			return 0, errors.Wrapf(ErrDepInvalidImport, "bad contract name in %q", raw)
		}

		return getNumByName(txn, deployer, name)
	}

	num, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(ErrDepInvalidImport, "bad dependency identifier %q", raw)
	}

	return num, nil
}

// extractContractDeps compiles the contract text and returns its
// declared dependencies resolved to account nums (for the lock-time
// bookkeeping)
func extractContractDeps(txn *bbolt.Tx, contractText []byte) ([]uint64, error) {
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

	deps, err := parseContractDeps(module)
	if err != nil {
		return nil, err
	}

	return resolveDepNums(txn, deps)
}

// contractDep is one declared dependency: a library (code on the
// caller's state) or a service (code on its own state). Raw is the
// identifier as written in the import ("700" or "<bs58>.<name>")
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
		var numStr string
		var service bool
		switch {
		case strings.HasPrefix(imp.Module, ContractDepPrefix):
			numStr = imp.Module[len(ContractDepPrefix):]
		case strings.HasPrefix(imp.Module, ServiceDepPrefix):
			numStr = imp.Module[len(ServiceDepPrefix):]
			service = true
		default:
			continue
		}

		if !seen[imp.Module] {
			seen[imp.Module] = true
			deps = append(deps, contractDep{Raw: numStr, Service: service})
		}
	}

	if len(deps) > maxDepsPerContract {
		return nil, errors.Wrapf(ErrDepLimit, "%d imports exceed the cap %d", len(deps), maxDepsPerContract)
	}

	return deps, nil
}

// resolveDepNums flattens the dependency set into the unique account
// nums the reference ledger tracks
func resolveDepNums(txn *bbolt.Tx, deps []contractDep) ([]uint64, error) {
	seen := make(map[uint64]bool)
	nums := make([]uint64, 0, len(deps))
	for _, dep := range deps {
		num, err := resolveDepNum(txn, dep.Raw)
		if err != nil {
			return nil, err
		}
		if !seen[num] {
			seen[num] = true
			nums = append(nums, num)
		}
	}
	return nums, nil
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
