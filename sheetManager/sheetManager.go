package sheetManager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"math/big"
	"sync"
)

var log = logging.MustGetLogger("sheetManager")

type SheetManager struct {
	currentVault *ngtypes.Vault
	accounts     *sync.Map //map[uint64]*ngtypes.Account // TODO: sync.Map
	anonymous    *sync.Map //map[string]*big.Int  // TODO: sync.Map
}

func NewSheetManager() *SheetManager {
	s := &SheetManager{
		currentVault: nil,
		accounts:     &sync.Map{},
		anonymous:    &sync.Map{},
	}

	return s
}

func (sm *SheetManager) Init(currentVault *ngtypes.Vault) {
	sm.currentVault = currentVault
}

func (sm *SheetManager) GetBalance(accountID uint64) (*big.Int, error) {
	account, exists := sm.accounts.Load(accountID)
	if !exists {
		return nil, ngtypes.ErrAccountNotExists
	}

	pk := hex.EncodeToString(account.(*ngtypes.Account).Owner)
	balance, exists := sm.anonymous.Load(pk)
	if !exists {
		return nil, ngtypes.ErrAccountBalanceNotExists
	}
	return balance.(*big.Int), nil
}

// TODO
// CheckTx will check the influenced accounts which mentioned in op, and verify their balance and nonce
func (sm *SheetManager) CheckTx(tx *ngtypes.Transaction) error {
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

// ApplyOpTransaction will apply the op into the balanceSheet if op is VALID
// TODO: !important
func (sm *SheetManager) ApplyTx(tx *ngtypes.Transaction) error {
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
		err = errors.New("unknown operation type")
	}

	return err
}

// ApplyBlockTxs will apply all txs in block to balanceSheet
func (sm *SheetManager) ApplyBlockTxs(b *ngtypes.Block) {
	for _, tx := range b.Transactions {
		err := sm.ApplyTx(tx)
		if err != nil {
			log.Panic(err)
		}
	}
}

// ApplyVault will apply list and delists in vault to balanceSheet
func (sm *SheetManager) ApplyVault(v *ngtypes.Vault) error {
	ok := sm.RegisterAccount(v.List)
	if !ok {
		return errors.New("failed to register account")
	}

	for i := range v.Delists {
		sm.DeleteAccount(v.Delists[i])
	}

	return nil
}

// called when mined Vault
func (sm *SheetManager) GetSheetBytes() []byte {
	sheet := sm.GenerateSheet()
	b, err := proto.Marshal(sheet)
	if err != nil {
		panic(err)
	}
	return b
}

// RegisterAccount is same to balanceSheet RegisterAccount, this is for consensus calling
func (sm *SheetManager) RegisterAccount(account *ngtypes.Account) (ok bool) {
	if _, exists := sm.accounts.Load(account.ID); !exists {
		sm.accounts.Store(account.ID, account)
		return true
	}

	return false
}

func (sm *SheetManager) DeleteAccount(account *ngtypes.Account) (ok bool) {
	if _, exists := sm.accounts.Load(account.ID); !exists {
		return false
	}

	sm.accounts.Delete(account.ID)
	return true
}

func (sm *SheetManager) AccountIsRegistered(accountID uint64) bool {
	_, exists := sm.accounts.Load(accountID)
	return exists
}

func (sm *SheetManager) GenerateSheet() *ngtypes.Sheet {
	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	sm.accounts.Range(func(height, account interface{}) bool {
		accounts[height.(uint64)] = account.(*ngtypes.Account)
		return true
	})

	sm.anonymous.Range(func(hexPK, balance interface{}) bool {
		anonymous[hexPK.(string)] = balance.(*big.Int).Bytes()
		return true
	})

	return ngtypes.NewSheet(accounts, anonymous)
}
