package ngsheet

import (
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CheckTxs will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (m *sheetEntry) CheckTxs(txs ...*ngtypes.Tx) error {
	m.RLock()
	defer m.RUnlock()

	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		// check tx is signed
		if !tx.IsSigned() {
			return ngtypes.ErrTxIsNotSigned
		}

		switch tx.GetType() {
		case ngtypes.TX_GENERATE: // generate
			if err := m.CheckGenerate(tx); err != nil {
				return err
			}

		case ngtypes.TX_REGISTER: // register
			if err := m.CheckRegister(tx); err != nil {
				return err
			}

		case ngtypes.TX_LOGOUT: // register
			if err := m.CheckLogout(tx); err != nil {
				return err
			}

		case ngtypes.TX_TRANSACTION: // transaction
			if err := m.CheckTransaction(tx); err != nil {
				return err
			}

		case ngtypes.TX_ASSIGN: // assign & append
			if err := m.CheckAssign(tx); err != nil {
				return err
			}

		case ngtypes.TX_APPEND: // assign & append
			if err := m.CheckAppend(tx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *sheetEntry) CheckGenerate(generateTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[generateTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = generateTx.CheckGenerate(); err != nil {
		return err
	}

	// DO NOT CHECK BALANCE

	// check nonce
	if generateTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong generate nonce: %d, should be %d", generateTx.GetNonce(), convener.Nonce+1)
	}

	return nil
}

func (m *sheetEntry) CheckRegister(registerTx *ngtypes.Tx) error {
	// check structure and key
	if err := registerTx.CheckRegister(); err != nil {
		return err
	}

	// check balance
	payer := registerTx.GetParticipants()[0]
	rawPayerBalance, exists := m.anonymous[base58.FastBase58Encoding(payer)]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}
	payerBalance := new(big.Int).SetBytes(rawPayerBalance)

	expenditure := registerTx.TotalExpenditure()
	if payerBalance.Cmp(expenditure) < 0 {
		return fmt.Errorf("balance is insufficient for register")
	}

	// check nonce
	rawConvener, exists := m.accounts[registerTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}
	if registerTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong register nonce: %d, should be %d", registerTx.GetNonce(), convener.Nonce+1)
	}

	return nil
}

func (m *sheetEntry) CheckLogout(logoutTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[logoutTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = logoutTx.CheckLogout(utils.Bytes2PublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := logoutTx.TotalExpenditure()
	convenerBalance, err := m.GetBalanceByNum(logoutTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for logout")
	}

	// check nonce
	if logoutTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong logout nonce: %d, should be %d", logoutTx.GetNonce(), convener.Nonce+1)
	}

	return nil
}

func (m *sheetEntry) CheckTransaction(transactionTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[transactionTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = transactionTx.CheckTransaction(utils.Bytes2PublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := transactionTx.TotalExpenditure()
	convenerBalance, err := m.GetBalanceByNum(transactionTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for transaction")
	}

	// check nonce
	if transactionTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong transaction nonce: %d, should be %d", transactionTx.GetNonce(), convener.Nonce+1)
	}

	return nil
}

func (m *sheetEntry) CheckAssign(assignTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[assignTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = assignTx.CheckAssign(utils.Bytes2PublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := assignTx.TotalExpenditure()
	convenerBalance, err := m.GetBalanceByNum(assignTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for assign")
	}

	// check nonce
	if assignTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong assign nonce: %d, should be %d", assignTx.GetNonce(), convener.Nonce+1)
	}

	return nil
}

func (m *sheetEntry) CheckAppend(appendTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[appendTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = appendTx.CheckAppend(utils.Bytes2PublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := appendTx.TotalExpenditure()
	convenerBalance, err := m.GetBalanceByNum(appendTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for append")
	}

	// check nonce
	if appendTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong append nonce: %d, should be %d", appendTx.GetNonce(), convener.Nonce+1)
	}

	return nil
}
