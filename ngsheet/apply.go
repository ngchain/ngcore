package ngsheet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"github.com/ngchain/ngcore/ngtypes"
	"math/big"
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
			x, y := elliptic.Unmarshal(elliptic.P256(), raw)
			pk := ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     x,
				Y:     y,
			}
			if tx.Verify(pk) {
				i := 0
				participantBalance, exists := m.anonymous[hex.EncodeToString(tx.GetParticipants()[i])]
				if !exists {
					participantBalance = ngtypes.Big0
				}

				m.anonymous[hex.EncodeToString(tx.GetParticipants()[i])] = new(big.Int).Add(
					participantBalance,
					new(big.Int).SetBytes(tx.GetValues()[i]),
				)
			}

		case 1:
			convener, exists := m.accounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}
			x, y := elliptic.Unmarshal(elliptic.P256(), convener.Owner)
			pk := ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     x,
				Y:     y,
			}
			if tx.Verify(pk) {
				totalValue := ngtypes.Big0
				for i := range tx.GetValues() {
					totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
				}
				fee := new(big.Int).SetBytes(tx.GetFee())
				totalExpense := new(big.Int).Add(fee, totalValue)
				convener, exists := m.accounts[tx.GetConvener()]
				if !exists {
					return ngtypes.ErrAccountNotExists
				}
				convenerBalance, exists := m.anonymous[hex.EncodeToString(convener.Owner)]
				if !exists {
					return ngtypes.ErrAccountBalanceNotExists
				}
				if convenerBalance.Cmp(totalExpense) < 0 {
					return ngtypes.ErrTxBalanceInsufficient
				}

				//totalFee = totalFee.Add(totalFee, fee)

				m.anonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, totalExpense)

				for i := range tx.GetParticipants() {

					participantBalance, exists := m.anonymous[hex.EncodeToString(tx.GetParticipants()[i])]
					if !exists {
						participantBalance = ngtypes.Big0
					}

					m.anonymous[hex.EncodeToString(tx.GetParticipants()[i])] = new(big.Int).Add(
						participantBalance,
						new(big.Int).SetBytes(tx.GetValues()[i]),
					)
				}
			}

		case 2:
			// TODO: add state tx

		default:
			err = fmt.Errorf("unknown operation type")
		}

		if err != nil {
			return err
		}
	}
	return err
}
