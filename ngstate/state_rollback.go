package ngstate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/dgraph-io/badger/v2"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (state *State) rollback(txn *badger.Txn, block *ngtypes.Block) error {
	latestBlock, err := ngblocks.GetLatestBlock(txn)
	if err != nil {
		return err
	}
	if !bytes.Equal(block.Hash(), latestBlock.Hash()) {
		return fmt.Errorf("the block to be rollbacked must be the latest one")
	}

	// TODO: run reverse Txs
	panic("todo")

	return nil
}

func (state *State) reverseTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.GetType() {
		case ngtypes.TxType_INVALID:
			return fmt.Errorf("invalid tx")
		case ngtypes.TxType_GENERATE:
			if err := state.reverseGenerate(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_REGISTER:
			if err := state.reverseRegister(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_LOGOUT:
			if err := state.reverseLogout(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_TRANSACT:
			if err := state.reverseTransaction(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_APPEND: // append tx
			if err := state.reverseAppend(txn, tx); err != nil {
				return err
			}
		case ngtypes.TxType_DELETE: // delete tx
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

func (state *State) reverseRegister(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	balance, err := getBalance(txn, participants[0])
	if err != nil {
		return err
	}

	err = setBalance(txn, participants[0], new(big.Int).Add(balance, totalExpense))
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
	err = delOwnership(txn, participants[0])
	if err != nil {
		return err
	}

	return nil
}

func (state *State) reverseLogout(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	balance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Add(balance, totalExpense))
	if err != nil {
		return err
	}

	err = setAccount(txn, ngtypes.AccountNum(convener.Num), &ngtypes.Account{
		Num:      convener.Num,
		Owner:    tx.Extra, // logoutTx's extra as pub key
		Contract: nil,      // empty
		Context:  nil,      // empty
	})
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

func (state *State) reverseAppend(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Add(convenerBalance, fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var appendExtra ngtypes.AppendExtra
	err = proto.Unmarshal(tx.Extra, &appendExtra)
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

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}

func (state *State) reverseDelete(txn *badger.Txn, tx *ngtypes.Tx) (err error) {
	convener, err := getAccountByNum(txn, ngtypes.AccountNum(tx.GetConvener()))
	if err != nil {
		return err
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for deleteTx")
	}

	err = setBalance(txn, convener.Owner, new(big.Int).Add(convenerBalance, fee))
	if err != nil {
		return err
	}

	// append the extra bytes
	var deleteExtra ngtypes.DeleteExtra
	err = proto.Unmarshal(tx.Extra, &deleteExtra)
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

	err = setAccount(txn, ngtypes.AccountNum(tx.GetConvener()), convener)
	if err != nil {
		return err
	}

	return nil
}
