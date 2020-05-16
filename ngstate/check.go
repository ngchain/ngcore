package ngstate

import (
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CheckTxs will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (m *State) CheckTxs(txs ...*ngtypes.Tx) error {
	m.RLock()
	defer m.RUnlock()

	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		// check tx is signed
		if !tx.IsSigned() {
			return fmt.Errorf("tx is not signed")
		}

		if utils.Proto.Size(tx) > ngtypes.TxMaxExtraSize {
			return fmt.Errorf("tx is too large")
		}

		switch tx.GetType() {
		case ngtypes.TxType_GENERATE: // generate
			if err := m.CheckGenerate(tx); err != nil {
				return err
			}

		case ngtypes.TxType_REGISTER: // register
			if err := m.CheckRegister(tx); err != nil {
				return err
			}

		case ngtypes.TxType_LOGOUT: // logout
			if err := m.CheckLogout(tx); err != nil {
				return err
			}

		case ngtypes.TxType_TRANSACTION: // transaction
			if err := m.CheckTransaction(tx); err != nil {
				return err
			}

		case ngtypes.TxType_ASSIGN: // assign & append
			if err := m.CheckAssign(tx); err != nil {
				return err
			}

		case ngtypes.TxType_APPEND: // assign & append
			if err := m.CheckAppend(tx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *State) CheckGenerate(generateTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[generateTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = generateTx.CheckGenerate(); err != nil {
		return err
	}

	// DO NOT CHECK BALANCE

	// check nonce
	if generateTx.GetN() != convener.Txn+1 {
		return fmt.Errorf("wrong N: %d, should be %d", generateTx.GetN(), convener.Txn+1)
	}

	return nil
}

func (m *State) CheckRegister(registerTx *ngtypes.Tx) error {
	// check structure and key
	if err := registerTx.CheckRegister(); err != nil {
		return err
	}

	// check balance
	payer := registerTx.GetParticipants()[0]
	rawPayerBalance, exists := m.anonymous[base58.FastBase58Encoding(payer)]
	if !exists {
		return fmt.Errorf("account does not exist")
	}
	payerBalance := new(big.Int).SetBytes(rawPayerBalance)

	expenditure := registerTx.TotalExpenditure()
	if payerBalance.Cmp(expenditure) < 0 {
		return fmt.Errorf("balance is insufficient for register")
	}

	// check nonce
	rawConvener, exists := m.accounts[registerTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}
	if registerTx.GetN() != convener.Txn+1 {
		return fmt.Errorf("wrong N: %d, should be %d", registerTx.GetN(), convener.Txn+1)
	}

	return nil
}

func (m *State) CheckLogout(logoutTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[logoutTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
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
	if logoutTx.GetN() != convener.Txn+1 {
		return fmt.Errorf("wrong N: %d, should be %d", logoutTx.GetN(), convener.Txn+1)
	}

	return nil
}

func (m *State) CheckTransaction(transactionTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[transactionTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
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
	if transactionTx.GetN() != convener.Txn {
		return fmt.Errorf("wrong N: %d, should be %d", transactionTx.GetN(), convener.Txn)
	}

	return nil
}

func (m *State) CheckAssign(assignTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[assignTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
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
	if assignTx.GetN() != convener.Txn+1 {
		return fmt.Errorf("wrong assign nonce: %d, should be %d", assignTx.GetN(), convener.Txn+1)
	}

	return nil
}

func (m *State) CheckAppend(appendTx *ngtypes.Tx) error {
	rawConvener, exists := m.accounts[appendTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
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
	if appendTx.GetN() != convener.Txn+1 {
		return fmt.Errorf("wrong append nonce: %d, should be %d", appendTx.GetN(), convener.Txn+1)
	}

	return nil
}
