package ngstate

import (
	"encoding/binary"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleTxs will apply the tx into the state if tx is VALID
func HandleTxs(txn *badger.Txn, txs ...*ngtypes.Tx) (err error) {
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.GetType() {
		case ngtypes.TxType_INVALID:
			return fmt.Errorf("invalid tx")
		case ngtypes.TxType_GENERATE:
			if err := handleGenerate(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_REGISTER:
			if err := handleRegister(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_LOGOUT:
			if err := handleLogout(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_TRANSACTION:
			if err := handleTransaction(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_ASSIGN: // assign tx
			if err := handleAssign(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_APPEND: // append tx
			if err := handleAppend(txn, tx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown transaction type")
		}
	}

	return nil
}

func handleGenerate(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	publicKey := ngtypes.Address(tx.GetParticipants()[0]).PubKey()
	if err := tx.Verify(publicKey); err != nil {
		return err
	}

	participants := tx.GetParticipants()
	balance, err := getBalance(txn, participants[0])
	if err != nil {
		return err
	}

	err = setBalance(txn, participants[0], new(big.Int).Add(
		balance,
		new(big.Int).SetBytes(tx.GetValues()[0]),
	))
	if err != nil {
		return err
	}

	return nil
}

func handleRegister(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	log.Debugf("handling new register: %s", tx.BS58())
	publicKey := ngtypes.Address(tx.GetParticipants()[0]).PubKey()
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	balance, err := getBalance(txn, participants[0])
	if err != nil {
		return err
	}

	if balance.Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for register")
	}

	err = setBalance(txn, participants[0], new(big.Int).Sub(balance, totalExpense))
	if err != nil {
		return err
	}

	newAccount := ngtypes.NewAccount(ngtypes.AccountNum(binary.LittleEndian.Uint64(tx.GetExtra())), tx.GetParticipants()[0], nil, nil)

	num := ngtypes.AccountNum(newAccount.Num)
	err = setAccount(txn, num, newAccount)
	if err != nil {
		return err
	}

	// write ownership
	err = setOwnership(txn, participants[0], num)
	if err != nil {
		return err
	}

	return nil
}

func handleLogout(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()
	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	balance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if err = tx.Verify(pk); err != nil {
		return err
	}

	if balance.Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for logout")
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

func handleTransaction(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())
	totalExpense := new(big.Int).Add(fee, totalValue)

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for transaction")
	}
	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, totalExpense))
	if err != nil {
		return err
	}

	participants := tx.GetParticipants()
	for i := range participants {
		participantBalance, err := getBalance(txn, participants[i])
		if err != nil {
			return err
		}

		err = setBalance(txn, participants[i], new(big.Int).Add(
			participantBalance,
			new(big.Int).SetBytes(tx.GetValues()[i]),
		))
		if err != nil {
			return err
		}

		if addrHasAccount(txn, participants[i]) {
			num, err := getAccountNumByAddr(txn, participants[i])
			if err != nil {
				return err
			}

			account, err := getAccountByNum(txn, num)
			if err != nil {
				return err
			}

			vm, err := NewVM(txn, account)
			if err != nil {
				return err
			}

			err = vm.InitBuiltInImports()
			if err != nil {
				return err
			}

			vm.Call(tx)
		}
	}

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}

func handleAssign(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for assign")
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, fee))
	if err != nil {
		return err
	}

	// assign the extra bytes
	convener.Contract = tx.GetExtra()

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}

func handleAppend(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for assign")
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	convener.Contract = utils.CombineBytes(convener.Contract, tx.GetExtra())

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}
