package consensus_test

import (
	"strings"
	"testing"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

func TestNewConsensusManager(t *testing.T) {
	key := keytools.ReadLocalKey("ngcore.key", strings.TrimSpace(""))
	keytools.PrintPublicKey(key)

	db := storage.InitMemStorage()

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := ngchain.NewChain(db)
	chain.InitWithGenesis()

	sheetManager := ngsheet.GetSheetManager()
	txPool := txpool.NewTxPool(sheetManager)

	c := consensus.GetConsensus()
	c.Init(true, chain, sheetManager, key, txPool)
}
