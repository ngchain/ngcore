package jsonrpc_test

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestPrivateTxRelay proves the fire-and-forget commit-reveal flow: the wallet
// submits ONE ng_sendPrivateTx (commitment + one signed reveal) and never
// touches the reveal again. The node relays it — retargeting the height with
// no re-signing (effect-tx signatures are height-independent) — so it lands a
// block after its commitment, purely on tip movements.
func TestPrivateTxRelay(t *testing.T) {
	node := newRPCNode(t)

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	mineViaRPC(t, node, key) // fund the sender @1

	var to ngtypes.Address
	to[0] = 0x42

	// gen an unsigned transfer of 1 NG
	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to": to.BS58(), "value": "1", "fee": "0.001",
	}), &unsigned)

	raw, err := hex.DecodeString(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	var reveal ngtypes.FullTx
	if err := rlp.DecodeBytes(raw, &reveal); err != nil {
		t.Fatal(err)
	}

	// client-side seal: salt + sign the reveal ONCE (height-independent), then
	// build + sign the commitment riding the next block
	reveal.Salt = []byte("relay-mempool-salt-0123456789")
	if err := reveal.Signature(key); err != nil {
		t.Fatal(err)
	}

	tip := node.pow.Chain.GetLatestBlockHeight()
	buf := append(append([]byte{}, reveal.UnheightedHash()...), reveal.Salt...)
	commit := ngtypes.NewCommitment(ngtypes.ZERONET, tip+1, utils.Hash256(buf), big.NewInt(100_000_000_000_000))
	if err := commit.Signature(key); err != nil {
		t.Fatal(err)
	}

	commitRaw, err := rlp.EncodeToBytes(commit)
	if err != nil {
		t.Fatal(err)
	}
	revealRaw, err := rlp.EncodeToBytes(&reveal)
	if err != nil {
		t.Fatal(err)
	}

	// ONE call — after this the wallet is done, no ng_sendTx for the reveal
	node.mustCall(t, "ng_sendPrivateTx", map[string]any{
		"rawCommitment": hex.EncodeToString(commitRaw),
		"rawReveal":     hex.EncodeToString(revealRaw),
	})

	// mine the commit block; the tip move drives the relay to admit the reveal
	mineViaRPC(t, node, key)

	// the relay drains asynchronously off the tip hook — wait for it to admit
	if !waitFor(2*time.Second, func() bool { return len(node.pow.Pool.List()) > 0 }) {
		t.Fatal("relay never admitted the reveal after its commitment landed")
	}

	// mine the reveal's own block
	mineViaRPC(t, node, key)

	// the recipient was paid without the wallet ever re-submitting the reveal
	var bal struct{ TotalBalance string }
	decodeInto(t, node.mustCall(t, "ng_getBalanceByAddress",
		map[string]any{"address": to.BS58()}), &bal)
	if want := ngtypes.NG.String(); bal.TotalBalance != want {
		t.Fatalf("recipient balance = %s, want %s (relayed reveal did not land)", bal.TotalBalance, want)
	}
}

// TestRelayRejectsForgedReveal proves the enqueue guard: a reveal whose
// signature is tampered is rejected outright, so a forged reveal cannot squat
// a relay-queue slot until its window lapses.
func TestRelayRejectsForgedReveal(t *testing.T) {
	node := newRPCNode(t)

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	mineViaRPC(t, node, key) // fund the sender @1

	var to ngtypes.Address
	to[0] = 0x42

	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to": to.BS58(), "value": "1", "fee": "0.001",
	}), &unsigned)

	raw, err := hex.DecodeString(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	var reveal ngtypes.FullTx
	if err := rlp.DecodeBytes(raw, &reveal); err != nil {
		t.Fatal(err)
	}
	reveal.Salt = []byte("relay-mempool-salt-0123456789")
	if err := reveal.Signature(key); err != nil {
		t.Fatal(err)
	}

	// corrupt a signature-body byte (past the envelope tag+scheme): a
	// recover envelope then either fails recovery or recovers a different,
	// UNFUNDED From — the enqueue gate rejects both
	reveal.Sign[10] ^= 0xff

	forged, err := rlp.EncodeToBytes(&reveal)
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := node.call(t, "ng_sendReveal", map[string]any{"rawTx": hex.EncodeToString(forged)}); rpcErr == nil {
		t.Fatal("relay must reject a reveal with a forged signature")
	}
}

// waitFor polls cond until it holds or the timeout elapses
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
