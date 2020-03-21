package sheet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"github.com/ngchain/ngcore/ngtypes"
	"math/big"
)

// ApplyBlockTxs will apply all txs in block to balanceSheet
func (sm *Manager) ApplyBlockTxs(b *ngtypes.Block) {
	for _, tx := range b.Transactions {
		err := sm.ApplyTx(tx)
		if err != nil {
			log.Panic(err)
		}
	}
}

// ApplyVault will apply list and delists in vault to balanceSheet
func (sm *Manager) ApplyVault(v *ngtypes.Vault) error {
	ok := sm.RegisterAccount(v.List)
	if !ok {
		return fmt.Errorf("failed to register account: %d", v.List.ID)
	}

	for i := range v.Delists {
		sm.DeleteAccount(v.Delists[i])
	}

	return nil
}

// ApplyOpTransaction will apply the op into the balanceSheet if op is VALID
// TODO: !important
func (sm *Manager) ApplyTx(tx *ngtypes.Transaction) error {
	err := sm.CheckTx(tx)
	if err != nil {
		return err
	}

	err = tx.Check()
	if err != nil {
		return err
	}

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
			participantBalance, exists := sm.anonymous.Load(hex.EncodeToString(tx.GetParticipants()[i]))
			if !exists {
				participantBalance = ngtypes.Big0
			}

			sm.anonymous.Store(hex.EncodeToString(tx.GetParticipants()[i]), new(big.Int).Add(
				participantBalance.(*big.Int),
				new(big.Int).SetBytes(tx.GetValues()[i])),
			)
		}

	case 1:
		convener, exists := sm.accounts.Load(tx.GetConvener())
		if !exists {
			return ngtypes.ErrAccountNotExists
		}
		x, y := elliptic.Unmarshal(elliptic.P256(), convener.(*ngtypes.Account).Owner)
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
			convener, exists := sm.accounts.Load(tx.GetConvener())
			if !exists {
				return ngtypes.ErrAccountNotExists
			}
			convenerBalance, exists := sm.anonymous.Load(hex.EncodeToString(convener.(*ngtypes.Account).Owner))
			if !exists {
				return ngtypes.ErrAccountBalanceNotExists
			}
			if convenerBalance.(*big.Int).Cmp(totalExpense) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			//totalFee = totalFee.Add(totalFee, fee)

			sm.anonymous.Store(
				hex.EncodeToString(convener.(*ngtypes.Account).Owner),
				new(big.Int).Sub(convenerBalance.(*big.Int), totalExpense),
			)

			for i := range tx.GetParticipants() {

				participantBalance, exists := sm.anonymous.Load(hex.EncodeToString(tx.GetParticipants()[i]))
				if !exists {
					participantBalance = ngtypes.Big0
				}

				sm.anonymous.Store(hex.EncodeToString(tx.GetParticipants()[i]), new(big.Int).Add(
					participantBalance.(*big.Int),
					new(big.Int).SetBytes(tx.GetValues()[i])),
				)
			}
		}

	case 2:
		// state

	default:
		err = fmt.Errorf("unknown operation type")
	}

	return err
}
