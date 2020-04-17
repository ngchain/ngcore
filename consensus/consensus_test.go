package consensus

import (
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

func TestNewConsensusManager(t *testing.T) {
	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(""))
	keytools.PrintPublicKey(key)

	var db *badger.DB
	db = storage.InitMemStorage()

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := ngchain.GetChain(db)
	chain.InitWithGenesis()
	sheetManager := ngsheet.GetSheetManager()
	txPool := txpool.GetTxPool(sheetManager)

	consensus := GetConsensus()
	consensus.Init(true, chain, sheetManager, key, txPool)
}
