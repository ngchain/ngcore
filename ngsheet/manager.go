package ngsheet

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.MustGetLogger("sheet")

type Sheet struct {
	// currentVault *ngtypes.Vault
	baseSheet    *sheetEntry // the sheet from Vault, acts as the recovery
	currentSheet *sheetEntry
}

func NewSheetManager() *Sheet {
	s := &Sheet{
		baseSheet:    nil,
		currentSheet: nil,
	}

	return s
}

func (m *Sheet) Init(currentVault *ngtypes.Vault, blocks ...*ngtypes.Block) {
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
		err = m.currentSheet.handleTxs(blocks[i].Txs...)
		if err != nil {
			panic(err)
		}
	}
}

func (m *Sheet) GetCurrentBalanceByID(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByID(id)
}

func (m *Sheet) CheckTxs(txs ...*ngtypes.Tx) error {
	return m.currentSheet.CheckTxs()
}

func (m *Sheet) HandleTxs(transactions ...*ngtypes.Tx) error {
	return m.currentSheet.handleTxs(transactions...)
}

func (m *Sheet) HandleVault(vault *ngtypes.Vault) error {
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

	return nil
}

func (m *Sheet) GenerateNewSheet() (*ngtypes.Sheet, error) {
	return m.currentSheet.ToSheet()
}

func (m *Sheet) GetAccountsByPublicKey(key []byte) ([]*ngtypes.Account, error) {
	return m.currentSheet.GetAccountsByPublicKey(key)
}

func (m *Sheet) GetBalanceByID(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByID(id)
}

func (m *Sheet) GetBalanceByPublicKey(pk []byte) (*big.Int, error) {
	return m.currentSheet.GetBalanceByPublicKey(pk)
}

func (m *Sheet) GetNextNonce(convener uint64) uint64 {
	return m.currentSheet.GetNextNonce(convener)
}
