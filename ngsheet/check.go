package ngsheet

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CheckTxs will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (m *sheetEntry) CheckTxs(txs ...*ngtypes.Transaction) error {
	// checkFrom
	// - check exist
	// - check sign(pk)
	// - check nonce
	// - check balance

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
			if err := m.CheckGeneration(tx); err != nil {
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

func (m *sheetEntry) CheckGeneration(generationTx *ngtypes.Transaction) error {
	if err := generationTx.CheckGeneration(); err != nil {
		return err
	}

	rawConvener, exists := m.accounts[generationTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	totalCharge := generationTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(generationTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = generationTx.CheckTx(publicKey); err != nil {
		return err
	}

	// check nonce
	if generationTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong tx nonce")
	}

	return nil
}

func (m *sheetEntry) CheckRegister(registerTx *ngtypes.Transaction) error {
	if err := registerTx.CheckRegister(); err != nil {
		return err
	}

	rawConvener, exists := m.accounts[registerTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	totalCharge := registerTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(registerTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = registerTx.CheckTx(publicKey); err != nil {
		return err
	}

	// check nonce
	if registerTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong tx nonce")
	}

	return nil
}

func (m *sheetEntry) CheckLogout(logoutTx *ngtypes.Transaction) error {
	if err := logoutTx.CheckRegister(); err != nil {
		return err
	}

	rawConvener, exists := m.accounts[logoutTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	totalCharge := logoutTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(logoutTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = logoutTx.CheckTx(publicKey); err != nil {
		return err
	}

	// check nonce
	if logoutTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong tx nonce")
	}

	return nil
}

func (m *sheetEntry) CheckAssign(assignTx *ngtypes.Transaction) error {
	rawConvener, exists := m.accounts[assignTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	totalCharge := assignTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(assignTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	if len(assignTx.Header.Participants) != 1 || !bytes.Equal(assignTx.Header.Participants[0], make([]byte, 0)) {
		return fmt.Errorf("an assignment should have only one participant: nil")
	}

	if len(assignTx.Header.Values) != 1 || !bytes.Equal(assignTx.Header.Values[0], ngtypes.GetBig0Bytes()) {
		return fmt.Errorf("an assignment should have only one value: 0")
	}

	publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = assignTx.CheckTx(publicKey); err != nil {
		return err
	}

	// check nonce
	if assignTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong tx nonce")
	}

	return nil
}

func (m *sheetEntry) CheckAppend(appendTx *ngtypes.Transaction) error {
	rawConvener, exists := m.accounts[appendTx.GetConvener()]
	if !exists {
		return ngtypes.ErrAccountNotExists
	}

	convener := new(ngtypes.Account)
	err := convener.Unmarshal(rawConvener)
	if err != nil {
		return err
	}

	totalCharge := appendTx.TotalCharge()
	convenerBalance, err := m.GetBalanceByID(appendTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ngtypes.ErrTxBalanceInsufficient
	}

	if len(appendTx.Header.Participants) != 1 || !bytes.Equal(appendTx.Header.Participants[0], make([]byte, 0)) {
		return fmt.Errorf("an assignment should have only one participant: nil")
	}

	if len(appendTx.Header.Values) != 1 || !bytes.Equal(appendTx.Header.Values[0], ngtypes.GetBig0Bytes()) {
		return fmt.Errorf("an assignment should have only one value: 0")
	}

	publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

	if err = appendTx.CheckTx(publicKey); err != nil {
		return err
	}

	// check nonce
	if appendTx.GetNonce() != convener.Nonce+1 {
		return fmt.Errorf("wrong tx nonce")
	}

	return nil
}
