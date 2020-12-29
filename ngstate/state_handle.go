package ngstate

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/dgraph-io/badger/v2"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleTxs will apply the tx into the state if tx is VALID
func (state *State) HandleTxs(txn *badger.Txn, txs ...*ngtypes.Tx) (err error) {
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.GetType() {
		case ngtypes.TxType_INVALID:
			return fmt.Errorf("invalid tx")
		case ngtypes.TxType_GENERATE:
			if err := state.handleGenerate(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_REGISTER:
			if err := state.handleRegister(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_LOGOUT:
			if err := state.handleLogout(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_TRANSACT:
			if err := state.handleTransaction(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_APPEND: // append tx
			if err := state.handleAppend(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_DELETE: // delete tx
			if err := state.handleDelete(txn, tx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown tx type")
		}
	}

	return nil
}

func (state *State) handleGenerate(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
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

func (state *State) handleRegister(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
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

func (state *State) handleLogout(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
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

func (state *State) handleTransaction(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := big.NewInt(0)
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

			vm := state.vms[num]

			err = vm.InitBuiltInImports()
			if err != nil {
				return err
			}

			ins, err := vm.Instantiate(tx)

			vm.CallOnTx(ins)
		}
	}

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleAppend(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := big.NewInt(0)
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for appendTx")
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var appendExtra ngtypes.AppendExtra
	err = proto.Unmarshal(tx.Extra, &appendExtra)
	if err != nil {
		return err
	}

	convener.Contract = utils.InsertBytes(convener.Contract, int(appendExtra.Pos), appendExtra.Content...)

	// TODO: migrate to Lock
	//account, err := getAccountByNum(txn, ngtypes.AccountNum(tx.Convener))
	//if err != nil {
	//	return err
	//}
	//vm, err := NewVM(txn, account)
	//if err != nil {
	//	return err
	//}
	//
	//state.vms[ngtypes.AccountNum(tx.Convener)] = vm

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) handleDelete(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := big.NewInt(0)
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for deleteTx")
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var deleteExtra ngtypes.DeleteExtra
	err = proto.Unmarshal(tx.Extra, &deleteExtra)
	if err != nil {
		return err
	}

	convener.Contract = utils.CutBytes(convener.Contract, int(deleteExtra.Pos), int(deleteExtra.Pos)+len(deleteExtra.Content))

	// TODO: migrate to Lock
	//account, err := getAccountByNum(txn, ngtypes.AccountNum(tx.Convener))
	//if err != nil {
	//	return err
	//}
	//vm, err := NewVM(txn, account)
	//if err != nil {
	//	return err
	//}
	//
	//state.vms[ngtypes.AccountNum(tx.Convener)] = vm

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}
