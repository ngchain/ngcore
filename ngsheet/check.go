package ngsheet

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CheckTxs will check the influenced accounts which mentioned in op, and verify their balance and nonce
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
		// check tx is signed
		if !tx.IsSigned() {
			return ngtypes.ErrTxIsNotSigned
		}

		switch tx.GetType() {
		case 0: // generation
			if err := tx.CheckGen(); err != nil {
				return err
			}
		case 1: // tx
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

			publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

			if err = tx.CheckTx(publicKey); err != nil {
				return err
			}

			// check nonce
			if convener.Nonce >= tx.GetNonce() {
				return fmt.Errorf("wrong tx nonce")
			}
		case 2, 3: // assignment
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

			if len(tx.Header.Participants) != 1 || !bytes.Equal(tx.Header.Participants[0], make([]byte, 0)) {
				return fmt.Errorf("an assignment should have only one participant: nil")
			}

			if len(tx.Header.Values) != 1 || !bytes.Equal(tx.Header.Values[0], ngtypes.Big0Bytes) {
				return fmt.Errorf("an assignment should have only one value: 0")
			}

			publicKey := utils.Bytes2ECDSAPublicKey(convener.Owner)

			if err = tx.CheckTx(publicKey); err != nil {
				return err
			}
		}
	}

	return nil
}
