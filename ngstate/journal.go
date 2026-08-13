package ngstate

import (
	"math/big"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// vmJournal buffers every state change a contract makes during one call.
// Reads go through the overlay first (read-your-writes), and nothing hits
// the db until flush, so a failed call leaves the chain state untouched
type vmJournal struct {
	self *ngtypes.Account

	// context is a deep copy of self.Context which the kv module mutates
	context *ngtypes.AccountContext

	// balances holds the pending absolute balance per address
	balances map[ngtypes.Address]*big.Int
}

func bigIntFromUint64(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}

func newVMJournal(self *ngtypes.Account) *vmJournal {
	return &vmJournal{
		self:     self,
		context:  self.Context.Clone(),
		balances: make(map[ngtypes.Address]*big.Int),
	}
}

// balanceOf reads the pending balance, falling back to the db state
func (j *vmJournal) balanceOf(txn *bbolt.Tx, addr ngtypes.Address) *big.Int {
	if pending, ok := j.balances[addr]; ok {
		return new(big.Int).Set(pending)
	}

	return getBalance(txn, addr)
}

// transfer moves value between two addresses inside the journal
func (j *vmJournal) transfer(txn *bbolt.Tx, from, to ngtypes.Address, value *big.Int) error {
	if value.Sign() < 0 {
		return ngtypes.ErrTxValuesInvalid
	}

	fromBalance := j.balanceOf(txn, from)
	if fromBalance.Cmp(value) < 0 {
		return ErrTxrBalanceInsufficient
	}

	j.balances[from] = new(big.Int).Sub(fromBalance, value)
	j.balances[to] = new(big.Int).Add(j.balanceOf(txn, to), value)

	return nil
}

// flush applies all pending changes onto the db txn
func (j *vmJournal) flush(txn *bbolt.Tx) error {
	for addr, balance := range j.balances {
		if err := setBalance(txn, addr, balance); err != nil {
			return err
		}
	}

	j.self.Context = j.context

	return setAccount(txn, ngtypes.AccountNum(j.self.Num), j.self)
}
