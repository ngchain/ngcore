package ngstate

import (
	"bytes"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestGenesisStateRootMatchesHeader pins the two independent genesis-root
// implementations together: ngtypes computes the genesis header's StateRoot
// from statetrie's in-memory store (no ngstate import, to avoid a cycle), while
// ngstate produces the root by actually applying the genesis block to a fresh
// state db through the real Upgrade path. They MUST agree, or a node would
// reject its own genesis.
func TestGenesisStateRootMatchesHeader(t *testing.T) {
	for _, network := range []ngtypes.Network{ngtypes.ZERONET, ngtypes.TESTNET} {
		db := newTestDB(t)
		state := InitStateFromGenesis(db, network)

		want := ngtypes.GetGenesisBlock(network).BlockHeader.StateRoot
		if len(want) != ngtypes.HashSize {
			t.Fatalf("%s: genesis header StateRoot is %d bytes", network, len(want))
		}

		var got []byte
		if err := state.DB.View(func(txn *bbolt.Tx) error {
			got = StateRoot(txn)
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("%s: applied genesis root %x != header %x", network, got, want)
		}
	}
}
