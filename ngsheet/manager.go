package ngsheet

import (
	"math/big"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("sheet")

// SheetManager is a SheetManager manager module
type SheetManager struct {
	// currentVault *ngtypes.Vault
	baseSheet    *sheetEntry // the sheet from Vault, acts as the recovery
	currentSheet *sheetEntry
}

var sheetManager *SheetManager

// NewSheetManager will create a Sheet manager
func NewSheetManager() *SheetManager {
	if sheetManager == nil {
		sheetManager = &SheetManager{
			baseSheet:    nil,
			currentSheet: nil,
		}
	}

	return sheetManager
}

// Init will initialize the Sheet manager with a specific vault and blocks on the vault
func (m *SheetManager) Init(latestBlocks *ngtypes.Block) {
	var err error

	m.baseSheet, err = newSheetEntry(latestBlocks.Sheet)
	if err != nil {
		panic(err)
	}
	m.currentSheet, err = newSheetEntry(latestBlocks.Sheet)
	if err != nil {
		panic(err)
	}
}

// GetBalanceByNum is a helper to call GetBalanceByNum from currentSheet
func (m *SheetManager) GetBalanceByNum(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByNum(id)
}

// CheckTxs is a helper to call CheckTxs from currentSheet
func (m *SheetManager) CheckTxs(txs ...*ngtypes.Tx) error {
	return m.currentSheet.CheckTxs()
}

func (m *SheetManager) HandleTxs(txs ...*ngtypes.Tx) error {
	log.Debugf("handling %d txs", len(txs))
	return m.currentSheet.handleTxs(txs...)
}

func (m *SheetManager) GenerateNewSheet() (*ngtypes.Sheet, error) {
	return m.currentSheet.ToSheet()
}

func (m *SheetManager) GetAccountsByPublicKey(key []byte) ([]*ngtypes.Account, error) {
	return m.currentSheet.GetAccountsByPublicKey(key)
}

func (m *SheetManager) GetAccountByNum(id uint64) (*ngtypes.Account, error) {
	return m.currentSheet.GetAccountByNum(id)
}

func (m *SheetManager) GetBalanceByPublicKey(pk []byte) (*big.Int, error) {
	return m.currentSheet.GetBalanceByPublicKey(pk)
}

func (m *SheetManager) GetNextNonce(convener uint64) uint64 {
	return m.currentSheet.GetNextNonce(convener)
}
