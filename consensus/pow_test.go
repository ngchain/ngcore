package consensus_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/storage"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestNewConsensusManager(t *testing.T) {
	key := keytools.NewLocalKey()

	db := storage.InitMemStorage()

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	chain := ngblocks.NewChain(db)
	chain.InitWithGenesis()

	_ = ngp2p.NewLocalNode(52520)

	err := db.View(func(txn *badger.Txn) error {
		return ngstate.Upgrade(txn, ngtypes.GetGenesisBlock())
	})
	if err != nil {
		panic(err)
	}

	consensus.InitPoWConsensus(1, key, true)
}
