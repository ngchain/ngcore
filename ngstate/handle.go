package ngstate

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleTxs will apply the tx into the sheet if tx is VALID
func (m *State) HandleTxs(txs ...*ngtypes.Tx) (err error) {
	err = m.CheckTxs(txs...)
	if err != nil {
		return err
	}

	m.Lock()
	defer m.Unlock()

	newAccounts := make(map[uint64][]byte)
	for i := range m.accounts {
		newAccounts[i] = make([]byte, len(m.accounts[i]))
		copy(newAccounts[i], m.accounts[i])
	}

	newAnonymous := make(map[string][]byte)
	for i := range m.anonymous {
		newAnonymous[i] = make([]byte, len(m.anonymous[i]))
		copy(newAnonymous[i], m.anonymous[i])
	}

	defer func() {
		if err == nil {
			m.accounts = newAccounts
			m.anonymous = newAnonymous
		}
	}()

	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.GetType() {
		case ngtypes.TxType_INVALID:
			return fmt.Errorf("invalid tx")
		case ngtypes.TxType_GENERATE:
			if err := handleGenerate(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TxType_REGISTER:
			if err := handleRegister(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TxType_LOGOUT:
			if err := handleLogout(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TxType_TRANSACTION:
			if err := handleTransaction(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TxType_ASSIGN: // assign tx
			if err := handleAssign(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TxType_APPEND: // append tx
			if err := handleAppend(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown transaction type")
		}
	}

	return err
}

func handleGenerate(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	if err := utils.Proto.Unmarshal(rawConvener, convener); err != nil {
		return err
	}

	raw := tx.GetParticipants()[0]
	publicKey := utils.Bytes2PublicKey(raw)
	if err := tx.Verify(publicKey); err != nil {
		return err
	}

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := anonymous[base58.FastBase58Encoding(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	anonymous[base58.FastBase58Encoding(participants[0])] = new(big.Int).Add(
		new(big.Int).SetBytes(rawParticipantBalance),
		new(big.Int).SetBytes(tx.GetValues()[0]),
	).Bytes()

	accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}

func handleRegister(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	log.Debugf("handling new register: %s", tx.BS58())
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	raw := tx.GetParticipants()[0]
	publicKey := utils.Bytes2PublicKey(raw)
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := anonymous[base58.FastBase58Encoding(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	if new(big.Int).SetBytes(rawParticipantBalance).Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for register")
	}
	anonymous[base58.FastBase58Encoding(participants[0])] = new(big.Int).Sub(
		new(big.Int).SetBytes(rawParticipantBalance),
		totalExpense,
	).Bytes()

	newAccount := ngtypes.NewAccount(binary.LittleEndian.Uint64(tx.GetExtra()), tx.GetParticipants()[0], nil, nil)
	if _, exists := accounts[newAccount.Num]; exists {
		return fmt.Errorf("failed to register account@%d", newAccount.Num)
	}

	accounts[newAccount.Num], err = utils.Proto.Marshal(newAccount)
	if err != nil {
		return err
	}

	accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}

func handleLogout(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	raw := tx.GetParticipants()[0]
	publicKey := utils.Bytes2PublicKey(raw)
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := anonymous[base58.FastBase58Encoding(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	if new(big.Int).SetBytes(rawParticipantBalance).Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for logout")
	}
	anonymous[base58.FastBase58Encoding(participants[0])] = new(big.Int).Sub(
		new(big.Int).SetBytes(rawParticipantBalance),
		totalExpense,
	).Bytes()

	rawAccount, exists := accounts[binary.LittleEndian.Uint64(tx.GetExtra())]
	if !exists {
		return fmt.Errorf("trying to logout an unregistered account")
	}

	delAccount := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawAccount, delAccount)
	if err != nil {
		return err
	}

	if _, exists := accounts[delAccount.Num]; !exists {

		return fmt.Errorf("failed to delete account@%d", delAccount.Num)
	}

	delete(accounts, delAccount.Num)

	return nil
}

func handleTransaction(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := utils.Bytes2PublicKey(convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())
	totalExpense := new(big.Int).Add(fee, totalValue)

	rawConvenerBalance, exists := anonymous[base58.FastBase58Encoding(convener.Owner)]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for transaction")
	}

	anonymous[base58.FastBase58Encoding(convener.Owner)] = new(big.Int).Sub(convenerBalance, totalExpense).Bytes()

	participants := tx.GetParticipants()
	for i := range participants {
		var rawParticipantBalance []byte
		rawParticipantBalance, exists = anonymous[base58.FastBase58Encoding(participants[i])]
		if !exists {
			rawParticipantBalance = ngtypes.GetBig0Bytes()
		}

		anonymous[base58.FastBase58Encoding(participants[i])] = new(big.Int).Add(
			new(big.Int).SetBytes(rawParticipantBalance),
			new(big.Int).SetBytes(tx.GetValues()[i]),
		).Bytes()
	}

	accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	// DO NOT handle extra
	// TODO: call vm's tx listener

	return nil
}

func handleAssign(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := utils.Bytes2PublicKey(convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	rawConvenerBalance, exists := anonymous[base58.FastBase58Encoding(convener.Owner)]
	if !exists {
		return fmt.Errorf("account balance does not exist")
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for assign")
	}

	anonymous[base58.FastBase58Encoding(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

	// assign the extra bytes
	convener.Contract = tx.GetExtra()

	accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}

func handleAppend(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := utils.Bytes2PublicKey(convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	rawConvenerBalance, exists := anonymous[base58.FastBase58Encoding(convener.Owner)]
	if !exists {
		return fmt.Errorf("account balance does not exist")
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for append")
	}

	anonymous[base58.FastBase58Encoding(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

	// append the extra bytes
	convener.Contract = utils.CombineBytes(convener.Contract, tx.GetExtra())
	accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}
