package txpool

import (
	"testing"

	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestNewTxPool(t *testing.T) {
	sheetManager := ngsheet.NewSheetManager()
	pool := NewTxPool(sheetManager)

	sheetManager.Init(ngtypes.GetGenesisVault(), ngtypes.GetGenesisBlock())
	pool.Init(ngtypes.GetGenesisVault(), nil, nil)
}
