package ngstate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/c0mm4nd/rlp"
	"math/big"

	"github.com/dgraph-io/badger/v3"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (state *State) rollback(txn *badger.Txn, block *ngtypes.Block) error {
	latestBlock, err := ngblocks.GetLatestBlock(txn)
	if err != nil {
		return err
	}
	if !bytes.Equal(block.GetHash(), latestBlock.GetHash()) {
		return fmt.Errorf("the block to be rollbacked must be the latest one")
	}

	// TODO: run reverse Txs
	panic("todo")

	//return nil
}

func (state *State) reverseTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.Type {
		case ngtypes.InvalidTx:
			return fmt.Errorf("invalid tx")
		case ngtypes.GenerateTx:
			if err := state.reverseGenerate(txn, tx); err != nil {
				return err
			}
		case ngtypes.RegisterTx:
			if err := state.reverseRegister(txn, tx); err != nil {
				return err
			}
		case ngtypes.DestroyTx:
			if err := state.reverseLogout(txn, tx); err != nil {
				return err
			}
		case ngtypes.TransactTx:
			if err := state.reverseTransaction(txn, tx); err != nil {
				return err
			}
		case ngtypes.AppendTx: // append tx
			if err := state.reverseAppend(txn, tx); err != nil {
				return err
			}
		case ngtypes.DeleteTx: // delete tx
			if err := state.reverseDelete(txn, tx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown tx type")
		}
	}

	return nil
}

func (state *State) reverseGenerate(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	participants := tx.Participants
	balance, err := getBalance(txn, participants[0])
	if err != nil {
		return err
	}

	err = setBalance(txn, participants[0], new(big.Int).Add(
		balance, tx.Values[0]))
	if err != nil {
		return err
	}

	return nil
}

func (state *State) reverseRegister(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	totalExpense := new(big.Int).Set(tx.Fee)

	participants := tx.Participants
	balance, err := getBalance(txn, participants[0])
	if err != nil {
		return err
	}

	err = setBalance(txn, participants[0], new(big.Int).Add(balance, totalExpense))
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
	err = delOwnership(txn, participants[0])
	if err != nil {
		return err
	}

	return nil
}

func (state *State) reverseLogout(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	totalExpense := new(big.Int).Set(tx.Fee)

	balance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Add(balance, totalExpense))
	if err != nil {
		return err
	}

	err = setAccount(txn, ngtypes.AccountNum(convener.Num), ngtypes.NewAccount(
		ngtypes.AccountNum(convener.Num),
		tx.Extra, // logoutTx's extra as pub key
		nil,      // empty
		nil,      // empty
	))
	if err != nil {
		return err
	}

	// remove ownership
	err = setOwnership(txn, convener.Owner, ngtypes.AccountNum(convener.Num))
	if err != nil {
		return err
	}

	return nil
}

// FIXME: cannot Transact Tx yet.
func (state *State) reverseTransaction(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
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

	for i := range tx.Participants {
		participantBalance, err := getBalance(txn, tx.Participants[i])
		if err != nil {
			return err
		}

		err = setBalance(txn, tx.Participants[i], new(big.Int).Add(
			participantBalance,
			tx.Values[i],
		))
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

			vm.CallOnTx(ins)
		}
	}

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) reverseAppend(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Add(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var appendExtra ngtypes.AppendExtra
	err = rlp.DecodeBytes(tx.Extra, &appendExtra)
	if err != nil {
		return err
	}

	convener.Contract = utils.CutBytes(convener.Contract, int(appendExtra.Pos), int(appendExtra.Pos)+len(appendExtra.Content))

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

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) reverseDelete(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, tx.Convener)
	if err != nil {
		return err
	}

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(tx.Fee) < 0 {
		return fmt.Errorf("balance is insufficient for deleteTx")
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Add(convenerBalance, tx.Fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var deleteExtra ngtypes.DeleteExtra
	err = rlp.DecodeBytes(tx.Extra, &deleteExtra)
	if err != nil {
		return err
	}

	convener.Contract = utils.InsertBytes(convener.Contract, int(deleteExtra.Pos), deleteExtra.Content...)

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

	err = setAccount(txn, tx.Convener, convener)
	if err != nil {
		return err
	}

	return nil
}
