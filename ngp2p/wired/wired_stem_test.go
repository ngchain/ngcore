package wired

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
)

// stemTestTx builds a signed transact tx that passes the stem handler's
// stateless gates (network, envelope, signature).
func stemTestTx(t *testing.T) *ngtypes.FullTx {
	t.Helper()

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	var dest ngtypes.Address
	dest[0] = 0xee
	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		dest, big.NewInt(10), big.NewInt(1), nil, nil)
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}

	return tx
}

func stemTestCommit(t *testing.T) *ngtypes.Commitment {
	t.Helper()

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	commit := ngtypes.NewCommitment(ngtypes.ZERONET, 1,
		bytes.Repeat([]byte{0xab}, ngtypes.HashSize), big.NewInt(0))
	if err := commit.Signature(key); err != nil {
		t.Fatal(err)
	}

	return commit
}

type stemReceipt[T any] struct {
	item T
	ttl  uint8
}

// TestWiredStemTxRoundtrip: a stem tx sent over the real wired protocol
// arrives at the peer's registered hook, signature-checked, with its TTL.
func TestWiredStemTxRoundtrip(t *testing.T) {
	fx := newWiredFixture(t, 0)

	got := make(chan stemReceipt[*ngtypes.FullTx], 1)
	fx.server.OnStemTx = func(tx *ngtypes.FullTx, ttl uint8) {
		got <- stemReceipt[*ngtypes.FullTx]{tx, ttl}
	}

	tx := stemTestTx(t)
	if err := fx.client.SendStemTx(fx.serverHost.ID(), 7, tx); err != nil {
		t.Fatal(err)
	}

	select {
	case r := <-got:
		if !bytes.Equal(r.item.GetHash(), tx.GetHash()) {
			t.Fatalf("stem tx hash mismatch: %x != %x", r.item.GetHash(), tx.GetHash())
		}
		if r.ttl != 7 {
			t.Fatalf("stem ttl = %d, want 7", r.ttl)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("stem tx never reached the peer's hook")
	}
}

// TestWiredStemCommitRoundtrip mirrors the tx round-trip for commitments.
func TestWiredStemCommitRoundtrip(t *testing.T) {
	fx := newWiredFixture(t, 0)

	got := make(chan stemReceipt[*ngtypes.Commitment], 1)
	fx.server.OnStemCommit = func(commit *ngtypes.Commitment, ttl uint8) {
		got <- stemReceipt[*ngtypes.Commitment]{commit, ttl}
	}

	commit := stemTestCommit(t)
	if err := fx.client.SendStemCommit(fx.serverHost.ID(), 3, commit); err != nil {
		t.Fatal(err)
	}

	select {
	case r := <-got:
		if !bytes.Equal(r.item.GetUnsignedHash(), commit.GetUnsignedHash()) {
			t.Fatal("stem commitment mismatch")
		}
		if r.ttl != 3 {
			t.Fatalf("stem ttl = %d, want 3", r.ttl)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("stem commitment never reached the peer's hook")
	}
}

// TestWiredStemSendUnknownPeer: the sender surfaces transport failures so
// the router can fall back to fluff.
func TestWiredStemSendUnknownPeer(t *testing.T) {
	fx := newWiredFixture(t, 0)

	if err := fx.client.SendStemTx(randomPeerID(t), 5, stemTestTx(t)); err == nil {
		t.Fatal("SendStemTx to an unknown peer must fail")
	}
	if err := fx.client.SendStemCommit(randomPeerID(t), 5, stemTestCommit(t)); err == nil {
		t.Fatal("SendStemCommit to an unknown peer must fail")
	}
}

// TestWiredStemTamperedRejected: a stem message whose signed header was
// tampered after signing fails wired verification — the hook never fires
// and the sender gets the standard reject.
func TestWiredStemTamperedRejected(t *testing.T) {
	fx := newWiredFixture(t, 0)

	fired := make(chan struct{}, 1)
	fx.server.OnStemTx = func(*ngtypes.FullTx, uint8) { fired <- struct{}{} }

	raw, err := rlp.EncodeToBytes(stemTestTx(t))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := rlp.EncodeToBytes(&StemPayload{TTL: 5, Raw: raw})
	if err != nil {
		t.Fatal(err)
	}

	id, stream := sendSigned(t, fx, StemTxMsg, payload, true) // tamper after signing

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}

	select {
	case <-fired:
		t.Fatal("a tampered stem message must not reach the hook")
	case <-time.After(300 * time.Millisecond):
	}
}

// TestWiredStemMalformedDroppedQuietly: garbage payloads, oversized
// frames, unsigned and wrong-network items are all dropped without a
// crash and without reaching the hook; the server keeps serving.
func TestWiredStemMalformedDroppedQuietly(t *testing.T) {
	fx := newWiredFixture(t, 0)

	fired := make(chan struct{}, 8)
	fx.server.OnStemTx = func(*ngtypes.FullTx, uint8) { fired <- struct{}{} }
	fx.server.OnStemCommit = func(*ngtypes.Commitment, uint8) { fired <- struct{}{} }

	encode := func(v interface{}) []byte {
		t.Helper()
		raw, err := rlp.EncodeToBytes(v)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	unsignedTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		ngtypes.Address{}, big.NewInt(1), big.NewInt(0), nil, nil)

	wrongNetTx := stemTestTx(t)
	wrongNetTx.Network = ngtypes.TESTNET

	cases := []struct {
		name    string
		msgType MsgType
		payload []byte
	}{
		{"garbage payload", StemTxMsg, []byte{0xff}},
		{"garbage inner tx", StemTxMsg, encode(&StemPayload{TTL: 5, Raw: []byte{0xff}})},
		{"unsigned tx", StemTxMsg, encode(&StemPayload{TTL: 5, Raw: encode(unsignedTx)})},
		{"wrong network tx", StemTxMsg, encode(&StemPayload{TTL: 5, Raw: encode(wrongNetTx)})},
		{"oversized frame", StemCommitMsg, encode(&StemPayload{TTL: 5, Raw: make([]byte, maxStemCommitWire+1)})},
		{"garbage inner commit", StemCommitMsg, encode(&StemPayload{TTL: 5, Raw: []byte{0xff}})},
	}

	for _, c := range cases {
		_, stream := sendSigned(t, fx, c.msgType, c.payload, false)
		_ = stream // fire-and-forget: no reply expected on the stem path
	}

	select {
	case <-fired:
		t.Fatal("a malformed stem message must not reach the hook")
	case <-time.After(500 * time.Millisecond):
	}

	// the server survived it all and still serves valid stem traffic
	tx := stemTestTx(t)
	if err := fx.client.SendStemTx(fx.serverHost.ID(), 1, tx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fired:
	case <-time.After(10 * time.Second):
		t.Fatal("server stopped serving stem messages after malformed input")
	}
}
