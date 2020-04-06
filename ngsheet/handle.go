package ngsheet

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleTxs will apply the tx into the sheet if tx is VALID
func (m *sheetEntry) handleTxs(txs ...*ngtypes.Tx) (err error) {
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
		case ngtypes.TX_INVALID:
			return fmt.Errorf("invalid tx")
		case ngtypes.TX_GENERATION:
			if err = handleGeneration(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TX_REGISTER:
			if err = handleRegister(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TX_LOGOUT:
			if err = handleLogout(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TX_TRANSACTION:
			if err = handleTransaction(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TX_ASSIGN: // assign tx
			if err = handleAssign(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		case ngtypes.TX_APPEND: // append tx
			if err = handleAppend(newAccounts, newAnonymous, tx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown transaction type")
		}
	}

	return err
}

func handleGeneration(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err = convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	raw := tx.GetParticipants()[0]
	publicKey := utils.Bytes2ECDSAPublicKey(raw)
	if err := tx.Verify(publicKey); err != nil {
		return err
	}

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := anonymous[hex.EncodeToString(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	anonymous[hex.EncodeToString(participants[0])] = new(big.Int).Add(
		new(big.Int).SetBytes(rawParticipantBalance),
		new(big.Int).SetBytes(tx.GetValues()[0]),
	).Bytes()

	convener.Nonce++
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	return nil
}

func handleRegister(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err = convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	raw := tx.GetParticipants()[0]
	publicKey := utils.Bytes2ECDSAPublicKey(raw)
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := anonymous[hex.EncodeToString(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	if new(big.Int).SetBytes(rawParticipantBalance).Cmp(totalExpense) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}
	anonymous[hex.EncodeToString(participants[0])] = new(big.Int).Sub(
		new(big.Int).SetBytes(rawParticipantBalance),
		totalExpense,
	).Bytes()

	newAccount := ngtypes.NewAccount(binary.LittleEndian.Uint64(tx.GetExtra()), tx.GetParticipants()[0], nil)
	if _, exists := accounts[newAccount.ID]; exists {
		return fmt.Errorf("failed to register account@%d", newAccount.ID)
	}

	accounts[newAccount.ID], err = newAccount.Marshal()
	if err != nil {
		return err
	}

	convener.Nonce++
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	return nil
}

func handleLogout(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	raw := tx.GetParticipants()[0]
	publicKey := utils.Bytes2ECDSAPublicKey(raw)
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := anonymous[hex.EncodeToString(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	if new(big.Int).SetBytes(rawParticipantBalance).Cmp(totalExpense) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}
	anonymous[hex.EncodeToString(participants[0])] = new(big.Int).Sub(
		new(big.Int).SetBytes(rawParticipantBalance),
		totalExpense,
	).Bytes()

	rawAccount, exists := accounts[binary.LittleEndian.Uint64(tx.GetExtra())]
	if !exists {
		return fmt.Errorf("trying to logout an unregistered account")
	}

	delAccount := new(ngtypes.Account)
	err = delAccount.Unmarshal(rawAccount)
	if err != nil {
		return err
	}

	if _, exists := accounts[delAccount.ID]; !exists {

		return fmt.Errorf("failed to delete account@%d", delAccount.ID)
	}

	delete(accounts, delAccount.ID)

	return nil
}

func handleTransaction(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err = convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	pk := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())
	totalExpense := new(big.Int).Add(fee, totalValue)

	rawConvenerBalance, exists := anonymous[hex.EncodeToString(convener.Owner)]
	if !exists {
		return ngtypes.ErrAccountBalanceNotExists
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(totalExpense) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, totalExpense).Bytes()

	participants := tx.GetParticipants()
	for i := range participants {
		var rawParticipantBalance []byte
		rawParticipantBalance, exists = anonymous[hex.EncodeToString(participants[i])]
		if !exists {
			rawParticipantBalance = ngtypes.GetBig0Bytes()
		}

		anonymous[hex.EncodeToString(participants[i])] = new(big.Int).Add(
			new(big.Int).SetBytes(rawParticipantBalance),
			new(big.Int).SetBytes(tx.GetValues()[i]),
		).Bytes()
	}

	convener.Nonce++
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	// DO NOT handle extra
	return nil
}

func handleAssign(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err = convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	pk := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	rawConvenerBalance, exists := anonymous[hex.EncodeToString(convener.Owner)]
	if !exists {
		return ngtypes.ErrAccountBalanceNotExists
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(fee) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

	// assign the extra bytes
	convener.State = tx.GetExtra()
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	convener.Nonce++
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	return nil
}

func handleAppend(accounts map[uint64][]byte, anonymous map[string][]byte, tx *ngtypes.Tx) (err error) {
	rawConvener, exists := accounts[tx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err = convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	pk := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	rawConvenerBalance, exists := anonymous[hex.EncodeToString(convener.Owner)]
	if !exists {
		return ngtypes.ErrAccountBalanceNotExists
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(fee) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

	// assign the extra bytes
	convener.State = utils.CombineBytes(convener.State, tx.GetExtra())
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	convener.Nonce++
	accounts[tx.GetConvener()], err = convener.Marshal()
	if err != nil {
		return err
	}

	return nil
}
