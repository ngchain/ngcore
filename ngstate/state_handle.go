package ngstate

import (
	"encoding/binary"
	"math/big"

	"github.com/c0mm4nd/dbolt"
	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleTxs will apply the tx into the state if tx is VALID
func (state *State) HandleTxs(txn *dbolt.Tx, txs ...*ngtypes.Tx) (err error) {
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
			if err := state.handleTransaction(txn, tx); err != nil {
				return err
			}
		case ngtypes.AppendTx: // append tx
			if err := state.handleAppend(txn, tx); err != nil {
				return err
			}
		case ngtypes.DeleteTx: // delete tx
			if err := state.handleDelete(txn, tx); err != nil {
				return err
			}
		default:
			return errors.Wrapf(ngtypes.ErrTxTypeInvalid, "unknown tx type %d", tx.Type)
		}
	}

	return nil
}

func (state *State) handleGenerate(txn *dbolt.Tx, tx *ngtypes.Tx) (err error) {
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

func (state *State) handleRegister(txn *dbolt.Tx, tx *ngtypes.Tx) (err error) {
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

func (state *State) handleDestroy(txn *dbolt.Tx, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()
	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalExpense := new(big.Int).Set(tx.Fee)

	balance := getBalance(txn, convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

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

func (state *State) handleTransaction(txn *dbolt.Tx, tx *ngtypes.Tx) (err error) {
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

			vm := state.vms[num]

			err = vm.InitBuiltInImports()
			if err != nil {
				return err
			}

			ins, err := vm.Instantiate(tx)
			if err != nil {
				return err
			}

			vm.CallOnTx(ins)
		}
	}

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleAppend(txn *dbolt.Tx, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(tx.Fee) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var appendExtra ngtypes.AppendExtra
	err = rlp.DecodeBytes(tx.Extra, &appendExtra)
	if err != nil {
		return err
	}

	convener.Contract = utils.InsertBytes(convener.Contract, int(appendExtra.Pos), appendExtra.Content...)

	// TODO: migrate to Lock
	// account, err := getAccountByNum(txn, ngtypes.AccountNum(tx.Convener))
	// if err != nil {
	//	return err
	// }
	// vm, err := NewVM(txn, account)
	// if err != nil {
	//	return err
	// }
	//
	// state.vms[ngtypes.AccountNum(tx.Convener)] = vm

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleDelete(txn *dbolt.Tx, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(tx.Fee) < 0 {
		return ErrTxrBalanceInsufficient
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var deleteExtra ngtypes.DeleteExtra
	err = rlp.DecodeBytes(tx.Extra, &deleteExtra)
	if err != nil {
		return err
	}

	convener.Contract = utils.CutBytes(convener.Contract, int(deleteExtra.Pos), int(deleteExtra.Pos)+len(deleteExtra.Content))

	// TODO: migrate to Lock
	// account, err := getAccountByNum(txn, ngtypes.AccountNum(tx.Convener))
	// if err != nil {
	//	return err
	// }
	// vm, err := NewVM(txn, account)
	// if err != nil {
	//	return err
	// }
	//
	// state.vms[ngtypes.AccountNum(tx.Convener)] = vm

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}
