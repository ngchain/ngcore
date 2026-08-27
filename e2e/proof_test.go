package e2e

import (
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/lightclient"
	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

// TestLightClientStateProof covers the p2p light-client loop end to end:
// node A (full, mining) funds an address; node B learns A's tip header via
// the normal block gossip; B then fetches a wired state proof from A and
// verifies it against the tip header it holds — trustless, no RPC trust.
// It checks an inclusion proof (a funded balance) and an absence proof (an
// untouched address).
func TestLightClientStateProof(t *testing.T) {
	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	key, _ := ngtypes.GenerateKey()
	minerAddr := ngtypes.NewAddress(key)

	// A mines two blocks funding the miner key; both gossip to B
	b1 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeB, b1.GetHash(), 10*time.Second)
	b2 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeB, b2.GetHash(), 10*time.Second)

	// the header B trusts is its own canonical tip (learned over gossip)
	tip := nodeB.chain.GetLatestBlock().(*ngtypes.FullBlock)
	header := tip.BlockHeader

	// the balance B expects, read from A's state for the assertion
	wantBal, err := nodeA.chain.State.GetTotalBalanceByAddress(minerAddr)
	if err != nil {
		t.Fatal(err)
	}
	if wantBal.Sign() == 0 {
		t.Fatal("miner balance should be non-zero after two blocks")
	}

	// --- inclusion proof: B asks A for the miner's balance at the tip ---
	resp, err := nodeB.local.RequestProof(nodeA.local.ID(), wired.ProofRequest{
		Domain: "balance",
		Key:    minerAddr[:],
		Height: 0,
	})
	if err != nil {
		t.Fatalf("RequestProof (balance): %v", err)
	}

	value, ok := lightclient.VerifyProof(header, resp)
	if !ok {
		t.Fatal("balance proof failed to verify against the trusted tip header")
	}
	if got := new(big.Int).SetBytes(value); got.Cmp(wantBal) != 0 {
		t.Fatalf("proven balance %s != want %s", got, wantBal)
	}

	// the proof must be bound to the very tip B trusts
	if resp.Height != header.GetHeight() {
		t.Fatalf("proof height %d != tip height %d", resp.Height, header.GetHeight())
	}

	// --- absence proof: an untouched address ---
	var absent ngtypes.Address
	for i := range absent {
		absent[i] = 0x5e
	}
	absResp, err := nodeB.local.RequestProof(nodeA.local.ID(), wired.ProofRequest{
		Domain: "balance",
		Key:    absent[:],
	})
	if err != nil {
		t.Fatalf("RequestProof (absence): %v", err)
	}
	if absResp.Found {
		t.Fatal("an untouched address must yield an absence proof")
	}
	absVal, ok := lightclient.VerifyProof(header, absResp)
	if !ok {
		t.Fatal("absence proof failed to verify")
	}
	if absVal != nil {
		t.Fatalf("absence proof returned a value: %x", absVal)
	}
}
