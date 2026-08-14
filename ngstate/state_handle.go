package ngstate

import (
	"encoding/binary"
	"math/big"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// HandleTxs will apply the tx into the state if tx is VALID.
// blockTime is the enclosing block's timestamp, exposed to contracts
func (state *State) HandleTxs(txn *bbolt.Tx, blockTime uint64, txs ...*ngtypes.FullTx) (err error) {
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.Type {
		case ngtypes.InvalidTx:
			return ngtypes.ErrTxTypeInvalid
		case ngtypes.GenerateTx:
			if err := state.handleGenerate(txn, tx); err != nil {
				return err
			}
		case ngtypes.RegisterTx:
			if err := state.handleRegister(txn, tx); err != nil {
				return err
			}
		case ngtypes.DestroyTx:
			if err := state.handleDestroy(txn, tx); err != nil {
				return err
			}
		case ngtypes.TransactTx:
			if err := state.handleTransaction(txn, tx, blockTime); err != nil {
				return err
			}
		case ngtypes.EditTx: // edit tx
			if err := state.handleEdit(txn, tx); err != nil {
				return err
			}
		case ngtypes.LockTx:
			if err := state.handleLock(txn, tx, blockTime); err != nil {
				return err
			}
		case ngtypes.UnlockTx:
			if err := state.handleUnlock(txn, tx); err != nil {
				return err
			}
		default:
			return errors.Wrapf(ngtypes.ErrTxTypeInvalid, "unknown tx type %d", tx.Type)
		}
	}

	return nil
}

func (state *State) handleGenerate(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	publicKey := tx.Participants[0].PubKey()
	if err := tx.Verify(publicKey); err != nil {
		return err
	}

	balance := getBalance(txn, tx.Participants[0])

	err = setBalance(txn, tx.Participants[0], new(big.Int).Add(balance, tx.Values[0]))
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleRegister(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	log.Debugf("handling new register: %s", tx.BS58())
	publicKey := tx.Participants[0].PubKey()
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).Set(tx.Fee)

	balance := getBalance(txn, tx.Participants[0])

	if balance.Cmp(totalExpense) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, tx.Participants[0], new(big.Int).Sub(balance, totalExpense))
	if err != nil {
		return err
	}

	newAccount := ngtypes.NewAccount(ngtypes.AccountNum(binary.LittleEndian.Uint64(tx.Extra)), tx.Participants[0], nil, nil)

	num := ngtypes.AccountNum(newAccount.Num)
	err = setAccount(txn, num, newAccount)
	if err != nil {
		return err
	}

	// write ownership
	err = setOwnership(txn, tx.Participants[0], num)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleDestroy(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()
	if err = tx.Verify(pk); err != nil {
		return err
	}

	// mirror checkDestroy: no destroying an account with a contract or
	// while locked (an active contract may be depended on downstream)
	if len(convener.Contract) != 0 {
		return ErrDestroyAccountContractNotEmpty
	}
	if convener.IsLocked() {
		return ErrAccountLocked
	}
	if refs := getRefCount(convener); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	// destroying releases the registered name
	if name := convener.Context.Get(contextKeyName); len(name) != 0 {
		if err := delContractName(txn, convener.Owner, string(name)); err != nil {
			return err
		}
	}

	totalExpense := new(big.Int).Set(tx.Fee)

	balance := getBalance(txn, convener.Owner)

	if balance.Cmp(totalExpense) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(balance, totalExpense))
	if err != nil {
		return err
	}

	err = delAccount(txn, ngtypes.AccountNum(convener.Num))
	if err != nil {
		return err
	}

	// remove ownership
	err = delOwnership(txn, convener.Owner)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleTransaction(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := big.NewInt(0)
	for i := range tx.Values {
		totalValue.Add(totalValue, tx.Values[i])
	}

	totalExpense := new(big.Int).Add(tx.Fee, totalValue)

	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(totalExpense) < 0 {
		return ErrTxrBalanceInsufficient
	}
	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, totalExpense))
	if err != nil {
		return err
	}

	// persist the convener BEFORE any contract runs: a contract on the
	// convener's own account would otherwise be overwritten by this
	// stale pre-vm copy
	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	for i := range tx.Participants {
		participantBalance := getBalance(txn, tx.Participants[i])

		err = setBalance(txn, tx.Participants[i], new(big.Int).Add(participantBalance, tx.Values[i]))
		if err != nil {
			return err
		}

		if addrHasAccount(txn, tx.Participants[i]) {
			num, err := getAccountNumByAddr(txn, tx.Participants[i])
			if err != nil {
				return err
			}

			state.runContract(txn, num, tx, VMEntryOnTx, blockTime)
		}
	}

	return nil
}

// handleEdit applies a whole patch (EditExtra hunks) onto the contract
// text atomically; the account must be unlocked
func (state *State) handleEdit(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.CheckEdit(pk); err != nil {
		return err
	}

	if convener.IsLocked() {
		return ErrAccountLocked
	}

	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(tx.Fee) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	editExtra, err := ngtypes.DecodeEditExtra(tx.Extra)
	if err != nil {
		return err
	}

	convener.Contract, err = editExtra.Apply(convener.Contract)
	if err != nil {
		return err
	}

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}

// handleLock freezes the contract of the convener account: the contract
// body becomes immutable and the vm gets active. The optional `init`
// export runs once here, right after locking
func (state *State) handleLock(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()
	if err = tx.CheckLock(pk); err != nil {
		return err
	}

	if convener.IsLocked() {
		return ErrAccountLocked
	}

	// locking activates the vm, so the contract text must compile
	if len(convener.Contract) != 0 {
		if _, err := CompileContract(convener.Contract); err != nil {
			return err
		}
	}

	convenerBalance := getBalance(txn, convener.Owner)
	if convenerBalance.Cmp(tx.Fee) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	// optional naming: a non-empty lock extra registers
	// <owner-address>.<name> as this contract's handle
	if len(tx.Extra) != 0 {
		name := string(tx.Extra)
		if !validContractName(name) {
			return errors.Wrapf(ErrNameInvalid, "%q", name)
		}
		if existing, err := getNumByName(txn, convener.Owner, name); err == nil && existing != uint64(tx.Convener) {
			return errors.Wrapf(ErrNameTaken, "%s -> account %d", name, existing)
		}
		if err := setContractName(txn, convener.Owner, name, uint64(tx.Convener)); err != nil {
			return err
		}
		convener.Context.Set(contextKeyName, []byte(name))
	}

	// module dependencies: every imported contract must be active, and
	// each dependee gets a reference pinned until this contract unlocks
	deps, err := extractContractDeps(txn, convener.Contract)
	if err != nil {
		return err
	}
	for _, num := range deps {
		if num == uint64(tx.Convener) {
			return ErrDepSelf
		}

		depAcc, err := getAccountByNum(txn, ngtypes.AccountNum(num))
		if err != nil {
			return errors.Wrapf(err, "unknown dependency contract %d", num)
		}
		if !depAcc.IsLocked() || len(depAcc.Contract) == 0 {
			return errors.Wrapf(ErrDepNotActive, "contract %d", num)
		}

		setRefCount(depAcc, getRefCount(depAcc)+1)
		if err := setAccount(txn, ngtypes.AccountNum(num), depAcc); err != nil {
			return err
		}
	}

	convener.SetLock(true)
	if err := setContractDeps(convener, deps); err != nil {
		return err
	}

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	state.runContract(txn, tx.Convener, tx, VMEntryOnLock, blockTime)

	return nil
}

// handleUnlock disables the vm of the convener account and makes the
// contract body editable again
func (state *State) handleUnlock(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()
	if err = tx.CheckUnlock(pk); err != nil {
		return err
	}

	if !convener.IsLocked() {
		return ErrAccountNotLocked
	}

	// a depended-on module cannot deactivate: its dependents would lose
	// the code they link against
	if refs := getRefCount(convener); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	convenerBalance := getBalance(txn, convener.Owner)
	if convenerBalance.Cmp(tx.Fee) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	// release the references this contract held on its dependencies
	deps, err := getContractDeps(convener)
	if err != nil {
		return err
	}
	for _, num := range deps {
		depAcc, err := getAccountByNum(txn, ngtypes.AccountNum(num))
		if err != nil {
			return err
		}
		if refs := getRefCount(depAcc); refs > 0 {
			setRefCount(depAcc, refs-1)
			if err := setAccount(txn, ngtypes.AccountNum(num), depAcc); err != nil {
				return err
			}
		}
	}

	convener.SetLock(false)
	if err := setContractDeps(convener, nil); err != nil {
		return err
	}

	return setAccount(txn, tx.Convener, convener)
}

// runContract executes the entry export of the account's contract, if the
// account is locked and has one. A contract failure is final for this call
// (its journal is dropped) but NEVER fails the tx itself: every node hits
// the same result, so consensus is kept
func (state *State) runContract(txn *bbolt.Tx, num ngtypes.AccountNum, tx *ngtypes.FullTx, entry string, blockTime uint64) {
	account, err := getAccountByNum(txn, num)
	if err != nil {
		log.Errorf("failed to load account %d for its contract: %v", num, err)
		return
	}

	if !account.IsLocked() || len(account.Contract) == 0 {
		return
	}

	vm, err := NewVM(txn, account, tx, blockTime)
	if err != nil {
		log.Errorf("failed to build the vm for account %d: %v", num, err)
		return
	}

	err = vm.Run(entry)
	if err != nil {
		if IsExportMissing(err) && entry != VMEntryOnTx {
			return // optional entry (e.g. init) is absent — fine
		}

		log.Errorf("contract call %s on account %d failed: %v", entry, num, err)
	}
}
