package consensus_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p"
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

	chain := storage.NewChain(db)
	chain.InitWithGenesis()

	sheetManager := ngsheet.GetSheetManager()
	txPool := txpool.NewTxPool(sheetManager)
	localNode := ngp2p.NewLocalNode(rand.Int(), false)

	consensus.NewConsensus(true, chain, sheetManager, key, txPool, localNode)
}
