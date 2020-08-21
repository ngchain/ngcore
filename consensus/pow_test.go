package consensus_test

import (
	"github.com/ngchain/ngcore/ngchain"
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

	ngblocks.Init(db)
	ngchain.Init(db)

	_ = ngp2p.NewLocalNode(52520)

	err := db.Update(func(txn *badger.Txn) error {
		return ngstate.Upgrade(txn, ngtypes.GetGenesisBlock())
	})
	if err != nil {
		panic(err)
	}

	consensus.InitPoWConsensus(1, key, true, db)
}
