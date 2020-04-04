package ngsheet

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.MustGetLogger("sheet")

type Manager struct {
	// currentVault *ngtypes.Vault
	baseSheet    *sheetEntry // the sheet from Vault, acts as the recovery
	currentSheet *sheetEntry
}

func NewSheetManager() *Manager {
	s := &Manager{
		baseSheet:    nil,
		currentSheet: nil,
	}

	return s
}

func (m *Manager) Init(currentVault *ngtypes.Vault, blocks ...*ngtypes.Block) {
	log.Infof("sheet manager initialized on vault@%d", currentVault.Height)

	var err error

	m.baseSheet, err = NewSheetEntry(currentVault.Sheet)
	if err != nil {
		panic(err)
	}
	m.currentSheet, err = NewSheetEntry(currentVault.Sheet)
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(blocks); i++ {
		err = m.currentSheet.HandleTxs(blocks[i].Transactions...)
		if err != nil {
			panic(err)
		}
	}
}

func (m *Manager) GetCurrentBalanceByID(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByID(id)
}

func (m *Manager) CheckTxs(txs ...*ngtypes.Transaction) error {
	return m.currentSheet.CheckTxs()
}

func (m *Manager) HandleTxs(transactions ...*ngtypes.Transaction) error {
	return m.currentSheet.HandleTxs(transactions...)
}

func (m *Manager) HandleVault(vault *ngtypes.Vault) error {
	newBaseSheetHash, err := vault.Sheet.CalculateHash()
	if err != nil {
		return err
	}

	currentSheet, err := m.currentSheet.ToSheet()
	if err != nil {
		return err
	}

	currentSheetHash, err := currentSheet.CalculateHash()
	if err != nil {
		return err
	}

	if !bytes.Equal(newBaseSheetHash, currentSheetHash) {
		return fmt.Errorf("malformed new sheet")
	}

	m.baseSheet = m.currentSheet
	m.currentSheet, err = NewSheetEntry(vault.Sheet)
	if err != nil {
		return err
	}

	return m.currentSheet.HandleVault(vault)
}

func (m *Manager) GenerateNewSheet() (*ngtypes.Sheet, error) {
	return m.currentSheet.ToSheet()
}

func (m *Manager) GetAccountsByPublicKey(key []byte) ([]*ngtypes.Account, error) {
	return m.currentSheet.GetAccountsByPublicKey(key)
}

func (m *Manager) GetBalanceByID(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByID(id)
}

func (m *Manager) GetNextNonce(convener uint64) uint64 {
	return m.currentSheet.GetNextNonce(convener)
}
