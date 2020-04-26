package txpool_test

import (
	"testing"

	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/txpool"
)

func TestNewTxPool(t *testing.T) {
	sheetManager := ngsheet.GetSheetManager()
	pool := txpool.NewTxPool(sheetManager)

	sheetManager.Init(ngtypes.GetGenesisBlock())
	pool.Init(nil)
}
