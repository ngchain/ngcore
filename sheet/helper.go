package sheet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"github.com/ngchain/ngcore/ngtypes"
)

// TODO
// CheckTx will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (sm *Manager) CheckTx(tx *ngtypes.Transaction) error {
	// checkFrom
	// - check exist
	// - check sign(pk)
	// - check nonce
	// - check balance
	if !tx.IsSigned() {
		return ngtypes.ErrTxIsNotSigned
	}

	switch tx.GetType() {
	case 0:
		x, y := elliptic.Unmarshal(elliptic.P256(), tx.GetParticipants()[0])
		publicKey := ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		}
		if !tx.Verify(publicKey) {
			return ngtypes.ErrTxWrongSign
		}
		return nil
	case 1:
		account, exists := sm.accounts.Load(tx.GetConvener())
		if !exists {
			return ngtypes.ErrAccountNotExists
		}
		convener := account.(*ngtypes.Account)

		totalCharge := tx.TotalCharge()
		convenerBalance, err := sm.GetBalance(tx.GetConvener())
		if err != nil {
			return err
		}

		if convenerBalance.Cmp(totalCharge) < 0 {
			return ngtypes.ErrTxBalanceInsufficient
		}

		// checkTo
		// - check exist
		for i := range tx.GetParticipants() {
			_, exists = sm.anonymous.Load(hex.EncodeToString(tx.GetParticipants()[i]))
			if !exists {
				return ngtypes.ErrAccountNotExists
			}
		}

		x, y := elliptic.Unmarshal(elliptic.P256(), convener.Owner)
		pubKey := ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		}
		if !tx.Verify(pubKey) {
			return ngtypes.ErrTxWrongSign
		}

		if convener.Nonce >= tx.GetNonce() {
			return ngtypes.ErrBlockNonceInvalid
		}

		return nil
	}

	return nil
}
