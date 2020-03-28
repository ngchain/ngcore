package ngsheet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"github.com/ngchain/ngcore/ngtypes"
)

// CheckTx will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (m *Manager) CheckTxs(txs ...*ngtypes.Transaction) error {
	// checkFrom
	// - check exist
	// - check sign(pk)
	// - check nonce
	// - check balance

	m.accountsMu.RLock()
	defer m.accountsMu.RUnlock()

	m.anonymousMu.RLock()
	defer m.anonymousMu.RUnlock()

	for _, tx := range txs {
		// check tx is sgined
		if !tx.IsSigned() {
			return ngtypes.ErrTxIsNotSigned
		}

		switch tx.GetType() {
		case 0:
			if err := tx.CheckGen(); err != nil {
				return err
			}
		case 1:
			// check convener exists
			convener, exists := m.accounts[tx.GetConvener()]
			if !exists {
				return ngtypes.ErrAccountNotExists
			}

			totalCharge := tx.TotalCharge()
			convenerBalance, err := m.GetBalance(tx.GetConvener())
			if err != nil {
				return err
			}

			if convenerBalance.Cmp(totalCharge) < 0 {
				return ngtypes.ErrTxBalanceInsufficient
			}

			x, y := elliptic.Unmarshal(elliptic.P256(), convener.Owner)
			pubKey := ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     x,
				Y:     y,
			}
			if err := tx.CheckTx(pubKey); err != nil {
				return err
			}

			// check nonce
			if convener.Nonce >= tx.GetNonce() {
				return ngtypes.ErrBlockNonceInvalid
			}
		}
	}

	return nil
}
