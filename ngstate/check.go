package ngstate

import (
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CheckTxs will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (s *State) CheckTxs(txs ...*ngtypes.Tx) error {
	s.RLock()
	defer s.RUnlock()

	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		// check tx is signed
		if !tx.IsSigned() {
			return fmt.Errorf("tx is not signed")
		}

		// check the tx's extra size is necessary
		if len(tx.Extra) > ngtypes.TxMaxExtraSize {
			return fmt.Errorf("tx is too large")
		}

		switch tx.GetType() {
		case ngtypes.TxType_GENERATE: // generate
			if err := s.CheckGenerate(tx); err != nil {
				return err
			}

		case ngtypes.TxType_REGISTER: // register
			if err := s.CheckRegister(tx); err != nil {
				return err
			}

		case ngtypes.TxType_LOGOUT: // logout
			if err := s.CheckLogout(tx); err != nil {
				return err
			}

		case ngtypes.TxType_TRANSACTION: // transaction
			if err := s.CheckTransaction(tx); err != nil {
				return err
			}

		case ngtypes.TxType_ASSIGN: // assign & append
			if err := s.CheckAssign(tx); err != nil {
				return err
			}

		case ngtypes.TxType_APPEND: // assign & append
			if err := s.CheckAppend(tx); err != nil {
				return err
			}
		}
	}

	return nil
}

// CheckGenerate checks the generate tx
func (s *State) CheckGenerate(generateTx *ngtypes.Tx) error {
	rawConvener, exists := s.accounts[generateTx.GetConvener()]
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

	return nil
}

// CheckRegister checks the register tx
func (s *State) CheckRegister(registerTx *ngtypes.Tx) error {
	// check structure and key
	if err := registerTx.CheckRegister(); err != nil {
		return err
	}

	// check balance
	payer := registerTx.GetParticipants()[0]
	rawPayerBalance, exists := s.anonymous[base58.FastBase58Encoding(payer)]
	if !exists {
		return fmt.Errorf("the payer for registering does not exist")
	}
	payerBalance := new(big.Int).SetBytes(rawPayerBalance)

	expenditure := registerTx.TotalExpenditure()
	if payerBalance.Cmp(expenditure) < 0 {
		return fmt.Errorf("balance is insufficient for register")
	}

	// check nonce
	rawConvener, exists := s.accounts[registerTx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err := utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	return nil
}

// CheckLogout checks logout tx
func (s *State) CheckLogout(logoutTx *ngtypes.Tx) error {
	rawConvener, exists := s.accounts[logoutTx.GetConvener()]
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
	convenerBalance, err := s.GetBalanceByNum(logoutTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for logout")
	}

	return nil
}

// CheckTransaction checks normal transaction tx
func (s *State) CheckTransaction(transactionTx *ngtypes.Tx) error {
	rawConvener, exists := s.accounts[transactionTx.GetConvener()]
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
	convenerBalance, err := s.GetBalanceByNum(transactionTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for transaction")
	}

	return nil
}

// CheckAssign checks assign tx
func (s *State) CheckAssign(assignTx *ngtypes.Tx) error {
	rawConvener, exists := s.accounts[assignTx.GetConvener()]
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
	convenerBalance, err := s.GetBalanceByNum(assignTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for assign")
	}

	return nil
}

// CheckAppend checks append tx
func (s *State) CheckAppend(appendTx *ngtypes.Tx) error {
	rawConvener, exists := s.accounts[appendTx.GetConvener()]
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
	convenerBalance, err := s.GetBalanceByNum(appendTx.GetConvener())
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return fmt.Errorf("balance is insufficient for append")
	}

	return nil
}
