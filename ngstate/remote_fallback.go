package ngstate

import (
	"math/big"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// remoteFallback is the OPTIONAL read-through hook lazy forking installs
// (`ngcore fork --rpc`): when a state read MISSES locally, the hook may
// supply that address's state from a remote node; it is then materialized
// into the local db (writable txns only), so every later read is local.
//
// nil — the default, and the only sane value on a validating node —
// changes nothing: this hook exists for the fork-chain debugging tool,
// never for consensus. The hook returns (contract-or-nil, balance-or-nil,
// found); it is expected to cache, including negative results.
var remoteFallback func(addr ngtypes.Address) (*ngtypes.Contract, *big.Int, bool)

// SetRemoteFallback installs the lazy-fork read-through hook (dev tooling
// only; see remoteFallback)
func SetRemoteFallback(f func(addr ngtypes.Address) (*ngtypes.Contract, *big.Int, bool)) {
	remoteFallback = f
}

// fetchRemoteState consults the hook on a local miss and materializes
// whatever it returns when the txn can write (execution paths run inside
// Update txns; read-only View paths still get the fetched value, it just
// stays unmaterialized until an execution touches the address)
func fetchRemoteState(txn *bbolt.Tx, addr ngtypes.Address) (*ngtypes.Contract, *big.Int, bool) {
	if remoteFallback == nil {
		return nil, nil, false
	}
	acc, bal, ok := remoteFallback(addr)
	if !ok {
		return nil, nil, false
	}
	if txn.Writable() {
		if acc != nil {
			_ = setContract(txn, acc)
		}
		if bal != nil && bal.Sign() > 0 {
			_ = setBalance(txn, addr, bal)
		}
	}
	return acc, bal, true
}
