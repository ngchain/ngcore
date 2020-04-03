package ngsheet

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleVault will apply list and delists in vault to balanceSheet
func (m *sheetEntry) HandleVault(v *ngtypes.Vault) error {
	if v.List != nil && v.List.ID != 0 {
		if ok := m.RegisterAccounts(v.List); !ok {
			return fmt.Errorf("failed to register accounts: %d", v.List.ID)
		}
	}

	if v.Delists != nil {
		if ok := m.DeleteAccounts(v.Delists...); !ok {
			return fmt.Errorf("failed to delist accounts: %v", v.Delists)
		}
	}

	return nil
}

// HandleTxs will apply the tx into the sheet if tx is VALID
func (m *sheetEntry) HandleTxs(txs ...*ngtypes.Transaction) (err error) {
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

	for _, tx := range txs {
		switch tx.GetType() {
		case 0:
			raw := tx.GetParticipants()[0]
			publicKey := utils.Bytes2ECDSAPublicKey(raw)
			if !tx.Verify(publicKey) {
				break
			}

			participants := tx.GetParticipants()
			rawParticipantBalance, exists := newAnonymous[hex.EncodeToString(participants[0])]
			if !exists {
				rawParticipantBalance = ngtypes.Big0Bytes
			}

			newAnonymous[hex.EncodeToString(participants[0])] = new(big.Int).Add(
				new(big.Int).SetBytes(rawParticipantBalance),
				new(big.Int).SetBytes(tx.GetValues()[0]),
			).Bytes()

		case 1:
			rawConvener, exists := newAccounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}

			convener := new(ngtypes.Account)
			err = convener.Unmarshal(rawConvener)
			if err != nil {
				return err
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

			rawConvenerBalance, exists := newAnonymous[hex.EncodeToString(convener.Owner)]
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}

			convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
			if convenerBalance.Cmp(totalExpense) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			newAnonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, totalExpense).Bytes()

			participants := tx.GetParticipants()
			for i := range participants {
				var rawParticipantBalance []byte
				rawParticipantBalance, exists = newAnonymous[hex.EncodeToString(participants[i])]
				if !exists {
					rawParticipantBalance = ngtypes.Big0Bytes
				}

				newAnonymous[hex.EncodeToString(participants[i])] = new(big.Int).Add(
					new(big.Int).SetBytes(rawParticipantBalance),
					new(big.Int).SetBytes(tx.GetValues()[i]),
				).Bytes()
			}

		case 2:
			// assignment tx
			rawConvener, exists := newAccounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}

			convener := new(ngtypes.Account)
			err = convener.Unmarshal(rawConvener)
			if err != nil {
				return err
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

			rawConvenerBalance, exists := newAnonymous[hex.EncodeToString(convener.Owner)]
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}

			convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
			if convenerBalance.Cmp(fee) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			newAnonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

			// assign the extra bytes
			convener.State = tx.GetExtra()
			newAccounts[tx.GetConvener()], err = convener.Marshal()
			if err != nil {
				return err
			}

		case 3:
			// append tx
			rawConvener, exists := newAccounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}

			convener := new(ngtypes.Account)
			err = convener.Unmarshal(rawConvener)
			if err != nil {
				return err
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

			rawConvenerBalance, exists := newAnonymous[hex.EncodeToString(convener.Owner)]
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}

			convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
			if convenerBalance.Cmp(fee) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			newAnonymous[hex.EncodeToString(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

			// assign the extra bytes
			convener.State = utils.CombineBytes(convener.State, tx.GetExtra())
			newAccounts[tx.GetConvener()], err = convener.Marshal()
			if err != nil {
				return err
			}

		default:
			err = fmt.Errorf("unknown operation type")
		}

		if err != nil {
			return err
		}
	}

	return err
}
