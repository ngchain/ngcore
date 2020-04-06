package ngsheet

import (
	"fmt"

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
		case ngtypes.TX_GENERATION: // generation
			if err := m.CheckGeneration(tx); err != nil {
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

func (m *sheetEntry) CheckGeneration(generationTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[generationTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = generationTx.CheckGenerate(); err != nil {
		return err
	}

	// DO NOT CHECK BALANCE

	// check nonce
	if generationTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong generation nonce: %d", generationTx.GetNonce())
	}

	return nil
}

func (m *sheetEntry) CheckRegister(registerTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[registerTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = registerTx.CheckRegister(); err != nil {
		return err
	}

	// check balance
	totalCharge := registerTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(registerTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	// check nonce
	if registerTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong register nonce: %d", registerTx.GetNonce())
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
	if err = logoutTx.CheckLogout(utils.Bytes2ECDSAPublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := logoutTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(logoutTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	// check nonce
	if logoutTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong logout nonce: %d", logoutTx.GetNonce())
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
	if err = transactionTx.CheckTransaction(utils.Bytes2ECDSAPublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := transactionTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(transactionTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	// check nonce
	if transactionTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong transaction nonce: %d", transactionTx.GetNonce())
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
	if err = assignTx.CheckAssign(utils.Bytes2ECDSAPublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := assignTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(assignTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	// check nonce
	if assignTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong assign nonce: %d", assignTx.GetNonce())
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
	if err = appendTx.CheckAppend(utils.Bytes2ECDSAPublicKey(convener.Owner)); err != nil {
		return err
	}

	// check balance
	totalCharge := appendTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(appendTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	// check nonce
	if appendTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong append nonce: %d", appendTx.GetNonce())
	}

	return nil
}
