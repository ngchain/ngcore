package ngsheet

import (
	"math/big"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.MustGetLogger("sheet")

// SheetManager is a SheetManager manager module
type SheetManager struct {
	// currentVault *ngtypes.Vault
	baseSheet    *sheetEntry // the sheet from Vault, acts as the recovery
	currentSheet *sheetEntry
}

// NewSheetManager will create a Sheet manager
func NewSheetManager() *SheetManager {
	s := &SheetManager{
		baseSheet:    nil,
		currentSheet: nil,
	}

	return s
}

// Init will initialize the Sheet manager with a specific vault and blocks on the vault
func (m *SheetManager) Init(currentVault *ngtypes.Vault, blocks ...*ngtypes.Block) {
	log.Infof("sheet manager initialized on vault@%d", currentVault.Height)

	var err error

	m.baseSheet, err = newSheetEntry(currentVault.Sheet)
	if err != nil {
		panic(err)
	}
	m.currentSheet, err = newSheetEntry(currentVault.Sheet)
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

// GetCurrentBalanceByID is a helper to call GetBalanceByID from currentSheet
func (m *SheetManager) GetCurrentBalanceByID(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByID(id)
}

// CheckCurrentTxs is a helper to call CheckTxs from currentSheet
func (m *SheetManager) CheckCurrentTxs(txs ...*ngtypes.Tx) error {
	return m.currentSheet.CheckTxs()
}

func (m *SheetManager) HandleTxs(transactions ...*ngtypes.Tx) error {
	return m.currentSheet.handleTxs(transactions...)
}

func (m *SheetManager) HandleVault(vault *ngtypes.Vault) (err error) {
	m.baseSheet = m.currentSheet
	m.currentSheet, err = newSheetEntry(vault.Sheet)
	if err != nil {
		return err
	}

	return nil
}

func (m *SheetManager) GenerateNewSheet() (*ngtypes.Sheet, error) {
	return m.currentSheet.ToSheet()
}

func (m *SheetManager) GetAccountsByPublicKey(key []byte) ([]*ngtypes.Account, error) {
	return m.currentSheet.GetAccountsByPublicKey(key)
}

func (m *SheetManager) GetAccountByID(id uint64) (*ngtypes.Account, error) {
	return m.currentSheet.GetAccountByID(id)
}

func (m *SheetManager) GetBalanceByID(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByID(id)
}

func (m *SheetManager) GetBalanceByPublicKey(pk []byte) (*big.Int, error) {
	return m.currentSheet.GetBalanceByPublicKey(pk)
}

func (m *SheetManager) GetNextNonce(convener uint64) uint64 {
	return m.currentSheet.GetNextNonce(convener)
}
