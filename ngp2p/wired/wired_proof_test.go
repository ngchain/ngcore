package wired

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
)

// tipHeader returns the fixture's canonical tip block header.
func tipHeader(t *testing.T, fx *wiredFixture) *ngtypes.BlockHeader {
	t.Helper()
	return fx.chain.GetLatestBlock().(*ngtypes.FullBlock).BlockHeader
}

// minerAddr recovers the miner address from a block's Coinbase field.
func minerAddr(fx *wiredFixture) ngtypes.Address {
	var addr ngtypes.Address
	copy(addr[:], fx.blocks[1].BlockHeader.Coinbase)
	return addr
}

// verifyAgainstHeader mirrors lightclient.VerifyProof (the wired package
// cannot import lightclient, which imports wired). It binds the proof to a
// trusted header and folds the branch into its StateRoot.
func verifyAgainstHeader(header *ngtypes.BlockHeader, resp *ProofResponse) (value []byte, ok bool) {
	if !bytes.Equal(resp.StateRoot, header.StateRoot) {
		return nil, false
	}
	if !bytes.Equal(resp.BlockHash, header.GetHash()) {
		return nil, false
	}

	var wantVH []byte
	if resp.Found {
		if len(resp.Value) == 0 {
			return nil, false
		}
		wantVH = statetrie.ValueHash(resp.Value)
	} else {
		if len(resp.Value) != 0 {
			return nil, false
		}
		wantVH = statetrie.ZeroHash()
	}
	if !bytes.Equal(resp.ValueHash, wantVH) {
		return nil, false
	}

	if !statetrie.Verify(resp.StateRoot, resp.Path, resp.ValueHash, resp.Proof) {
		return nil, false
	}
	if resp.Found {
		return resp.Value, true
	}
	return nil, true
}

// TestWiredProofBalanceRoundtrip: the client requests a balance proof for the
// miner (funded across mined blocks); the server serves it against the tip,
// and it verifies against the tip header, returning the right balance.
func TestWiredProofBalanceRoundtrip(t *testing.T) {
	fx := newWiredFixture(t, 3)

	// the miner in newWiredFixture funds itself each block; recover its
	// address + balance directly from the state to assert against
	miner := minerAddr(fx) // coinbase == miner address

	wantBal, err := fx.chain.State.GetTotalBalanceByAddress(miner)
	if err != nil {
		t.Fatal(err)
	}
	if wantBal.Sign() == 0 {
		t.Fatal("miner should have a non-zero balance after 3 blocks")
	}

	resp, err := fx.client.RequestProof(fx.serverHost.ID(), ProofRequest{
		Domain: "balance",
		Key:    miner[:],
		Height: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !resp.Found {
		t.Fatal("balance proof should be an inclusion proof")
	}

	header := tipHeader(t, fx)
	value, ok := verifyAgainstHeader(header, resp)
	if !ok {
		t.Fatal("valid proof rejected by verifier")
	}
	if got := new(big.Int).SetBytes(value); got.Cmp(wantBal) != 0 {
		t.Fatalf("proven balance %s != want %s", got, wantBal)
	}
}

// TestWiredProofAbsence: an untouched address yields a verifiable ABSENCE
// proof (Found=false, empty value).
func TestWiredProofAbsence(t *testing.T) {
	fx := newWiredFixture(t, 2)

	var absent ngtypes.Address
	for i := range absent {
		absent[i] = 0x7c
	}

	resp, err := fx.client.RequestProof(fx.serverHost.ID(), ProofRequest{
		Domain: "balance",
		Key:    absent[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Found {
		t.Fatal("an untouched address must yield an absence proof")
	}

	header := tipHeader(t, fx)
	value, ok := verifyAgainstHeader(header, resp)
	if !ok {
		t.Fatal("absence proof rejected by verifier")
	}
	if value != nil {
		t.Fatalf("absence proof returned a value: %x", value)
	}
}

// TestWiredProofTamperedRejected: a proof whose value was flipped after the
// server signed it no longer folds into the trusted root.
func TestWiredProofTamperedRejected(t *testing.T) {
	fx := newWiredFixture(t, 3)
	miner := minerAddr(fx)

	resp, err := fx.client.RequestProof(fx.serverHost.ID(), ProofRequest{
		Domain: "balance",
		Key:    miner[:],
	})
	if err != nil {
		t.Fatal(err)
	}

	header := tipHeader(t, fx)

	// (a) tamper the value: valueHash no longer matches
	bad := *resp
	bad.Value = append([]byte{}, resp.Value...)
	bad.Value[0] ^= 0xff
	if _, ok := verifyAgainstHeader(header, &bad); ok {
		t.Fatal("a tampered value must be rejected")
	}

	// (b) tamper a branch sibling: the fold no longer reaches the root
	bad2 := *resp
	bad2.Proof = make([][]byte, len(resp.Proof))
	copy(bad2.Proof, resp.Proof)
	bad2.Proof[0] = bytes.Repeat([]byte{0xaa}, statetrie.HashSize)
	if _, ok := verifyAgainstHeader(header, &bad2); ok {
		t.Fatal("a tampered branch must be rejected")
	}
}

// TestWiredProofWrongHeaderRejected: a valid proof bound to the tip does not
// verify against a DIFFERENT header (different StateRoot / hash).
func TestWiredProofWrongHeaderRejected(t *testing.T) {
	fx := newWiredFixture(t, 3)
	miner := minerAddr(fx)

	resp, err := fx.client.RequestProof(fx.serverHost.ID(), ProofRequest{
		Domain: "balance",
		Key:    miner[:],
	})
	if err != nil {
		t.Fatal(err)
	}

	// header at height 1 has a different StateRoot than the tip (height 3)
	wrong := fx.blocks[1].BlockHeader
	if _, ok := verifyAgainstHeader(wrong, resp); ok {
		t.Fatal("a proof must not verify against a header it was not bound to")
	}
}

// TestWiredProofBadPayload: a malformed proof request is rejected, not
// crashed (mirrors the getchain/getsheet reject paths).
func TestWiredProofBadPayload(t *testing.T) {
	fx := newWiredFixture(t, 1)

	id, stream := sendSigned(t, fx, GetProofMsg, []byte{0xff}, false)
	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

// TestWiredProofUnknownDomain: an unknown domain is rejected cleanly.
func TestWiredProofUnknownDomain(t *testing.T) {
	fx := newWiredFixture(t, 1)

	var addr ngtypes.Address
	if _, err := fx.client.RequestProof(fx.serverHost.ID(), ProofRequest{
		Domain: "bogus",
		Key:    addr[:],
	}); err == nil {
		t.Fatal("an unknown domain must be rejected")
	}
}

// TestWiredProofSendUnknownPeer: a request to an unreachable peer surfaces the
// transport error.
func TestWiredProofSendUnknownPeer(t *testing.T) {
	fx := newWiredFixture(t, 0)

	if _, _, err := fx.client.SendGetProof(randomPeerID(t), ProofRequest{Domain: "balance", Key: make([]byte, 32)}); err == nil {
		t.Fatal("SendGetProof to an unknown peer must fail")
	}
}
