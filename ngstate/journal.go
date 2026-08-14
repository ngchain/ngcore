package ngstate

import (
	"math/big"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// vmJournal buffers every state change one contract execution makes —
// across ALL touched accounts (service calls write to the callee's
// state). Reads go through the overlay first (read-your-writes), and
// nothing hits the db until flush, so a failed call leaves the chain
// state untouched
type vmJournal struct {
	// accounts are the loaded working copies, keyed by account num;
	// their Context fields are private clones the kv module mutates
	accounts map[uint64]*ngtypes.Account

	// balances holds the pending absolute balance per address
	balances map[ngtypes.Address]*big.Int
}

func bigIntFromUint64(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}

func newVMJournal(self *ngtypes.Account) *vmJournal {
	selfCopy := *self
	selfCopy.Context = self.Context.Clone()

	return &vmJournal{
		accounts: map[uint64]*ngtypes.Account{self.Num: &selfCopy},
		balances: make(map[ngtypes.Address]*big.Int),
	}
}

// accountOf returns the journal's working copy of the account, loading
// and cloning it on first touch
func (j *vmJournal) accountOf(txn *bbolt.Tx, num uint64) (*ngtypes.Account, error) {
	if acc, ok := j.accounts[num]; ok {
		return acc, nil
	}

	loaded, err := getAccountByNum(txn, ngtypes.AccountNum(num))
	if err != nil {
		return nil, err
	}

	copied := *loaded
	copied.Context = loaded.Context.Clone()
	j.accounts[num] = &copied

	return &copied, nil
}

// contextOf returns the journaled context of the account
func (j *vmJournal) contextOf(txn *bbolt.Tx, num uint64) (*ngtypes.AccountContext, error) {
	acc, err := j.accountOf(txn, num)
	if err != nil {
		return nil, err
	}

	return acc.Context, nil
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

	for num, acc := range j.accounts {
		if err := setAccount(txn, ngtypes.AccountNum(num), acc); err != nil {
			return errors.Wrapf(err, "failed to flush account %d", num)
		}
	}

	return nil
}
