package ngsheet

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ApplyVault will apply list and delists in vault to balanceSheet
func (m *Manager) ApplyVault(v *ngtypes.Vault) error {
	if v.List != nil && v.List.ID != 0 {
		if ok := m.RegisterAccount(v.List); !ok {
			return fmt.Errorf("failed to register account: %d", v.List.ID)
		}
	}

	if v.Delists != nil {
		for i := range v.Delists {
			m.DeleteAccount(v.Delists[i])
		}
	}

	return nil
}

// ApplyTxs will apply the tx into the sheet if tx is VALID
func (m *Manager) ApplyTxs(txs ...*ngtypes.Transaction) error {
	err := m.CheckTxs(txs...)
	if err != nil {
		return err
	}

	m.accountsMu.Lock()
	defer m.accountsMu.Unlock()

	m.anonymousMu.Lock()
	defer m.anonymousMu.Unlock()

	for _, tx := range txs {
		switch tx.GetType() {
		case 0:
			raw := tx.GetParticipants()[0]
			publicKey := utils.Bytes2ECDSAPublicKey(raw)
			if !tx.Verify(publicKey) {
				break
			}

			participants := tx.GetParticipants()
			participantBalance, exists := m.anonymous[hex.EncodeToString(participants[0])]
			if !exists {
				participantBalance = ngtypes.Big0
			}

			m.anonymous[hex.EncodeToString(participants[0])] = new(big.Int).Add(
				participantBalance,
				new(big.Int).SetBytes(tx.GetValues()[0]),
			)

		case 1:
			convener, exists := m.accounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}
			pk := utils.Bytes2ECDSAPublicKey(convener.Owner)

			if !tx.Verify(pk) {
				break
			}

			totalValue := ngtypes.Big0
			for i := range tx.GetValues() {
				totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
			}

			fee := new(big.Int).SetBytes(tx.GetFee())
			totalExpense := new(big.Int).Add(fee, totalValue)

			convenerBalance, exists := m.anonymous[hex.EncodeToString(convener.Owner)]
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}

			if convenerBalance.Cmp(totalExpense) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			m.anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, totalExpense)

			participants := tx.GetParticipants()
			for i := range participants {
				var participantBalance *big.Int

				participantBalance, exists = m.anonymous[hex.EncodeToString(participants[i])]
				if !exists {
					participantBalance = ngtypes.Big0
				}

				m.anonymous[hex.EncodeToString(participants[i])] = new(big.Int).Add(
					participantBalance,
					new(big.Int).SetBytes(tx.GetValues()[i]),
				)
			}

		case 2:
			// assignment tx
			convener, exists := m.accounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}
			pk := utils.Bytes2ECDSAPublicKey(convener.Owner)

			if !tx.Verify(pk) {
				break
			}

			totalValue := ngtypes.Big0
			for i := range tx.GetValues() {
				totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
			}

			fee := new(big.Int).SetBytes(tx.GetFee())

			convenerBalance, exists := m.anonymous[hex.EncodeToString(convener.Owner)]
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}

			if convenerBalance.Cmp(fee) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			m.anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee)

			// assign the extra bytes
			convener.State = tx.GetExtra()

		case 3:
			// append tx
			convener, exists := m.accounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}
			pk := utils.Bytes2ECDSAPublicKey(convener.Owner)

			if !tx.Verify(pk) {
				break
			}

			totalValue := ngtypes.Big0
			for i := range tx.GetValues() {
				totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
			}

			fee := new(big.Int).SetBytes(tx.GetFee())

			convenerBalance, exists := m.anonymous[hex.EncodeToString(convener.Owner)]
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}

			if convenerBalance.Cmp(fee) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			m.anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee)

			// assign the extra bytes
			convener.State = utils.CombineBytes(convener.State, tx.GetExtra())

		default:
			err = fmt.Errorf("unknown operation type")
		}

		if err != nil {
			return err
		}
	}
	return err
}
