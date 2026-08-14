package wired

import (
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"

	"github.com/ngchain/ngcore/ngtypes"
)

func newTestHost(t *testing.T) host.Host {
	t.Helper()

	h, err := libp2p.New(libp2p.NoListenAddrs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })

	return h
}

func signedMessage(t *testing.T, h host.Host, payload []byte) *Message {
	t.Helper()

	msg := &Message{
		Header:  NewHeader(h, ngtypes.ZERONET, []byte("test-id"), PingMsg),
		Payload: payload,
	}

	sign, err := Signature(h, msg)
	if err != nil {
		t.Fatal(err)
	}
	msg.Header.Sign = sign

	return msg
}

// TestMessageSignVerifyRoundtrip guards the sign/verify pair: the header
// must embed the key in the exact form the verifier parses. (A raw-vs-
// marshaled key mismatch here once broke ALL remote status exchanges and
// only surfaced in e2e)
func TestMessageSignVerifyRoundtrip(t *testing.T) {
	h := newTestHost(t)

	msg := signedMessage(t, h, []byte("hello"))

	if !Verify(h.ID(), msg) {
		t.Fatal("a properly signed message must verify")
	}

	// verify must not clobber the message (it temporarily strips Sign)
	if msg.Header.Sign == nil {
		t.Fatal("Verify must restore the signature")
	}
	if !Verify(h.ID(), msg) {
		t.Fatal("verification must be repeatable")
	}
}

func TestMessageVerifyRejectsTampering(t *testing.T) {
	h := newTestHost(t)

	// tampered payload
	msg := signedMessage(t, h, []byte("hello"))
	msg.Payload = []byte("evil!")
	if Verify(h.ID(), msg) {
		t.Fatal("tampered payload must not verify")
	}

	// tampered header field
	msg = signedMessage(t, h, []byte("hello"))
	msg.Header.Type = PongMsg
	if Verify(h.ID(), msg) {
		t.Fatal("tampered header must not verify")
	}

	// stripped signature
	msg = signedMessage(t, h, []byte("hello"))
	msg.Header.Sign = nil
	if Verify(h.ID(), msg) {
		t.Fatal("unsigned message must not verify")
	}
}

func TestMessageVerifyRejectsWrongPeer(t *testing.T) {
	alice := newTestHost(t)
	mallory := newTestHost(t)

	// a message signed by mallory must not verify as alice's
	msg := signedMessage(t, mallory, []byte("hello"))
	if Verify(alice.ID(), msg) {
		t.Fatal("message must not verify against another peer id")
	}

	// key substitution: mallory replaces the embedded key with her own
	// on a message claiming to come from alice
	msg = signedMessage(t, alice, []byte("hello"))
	evil := signedMessage(t, mallory, []byte("hello"))
	msg.Header.PeerKey = evil.Header.PeerKey
	msg.Header.Sign = evil.Header.Sign
	if Verify(alice.ID(), msg) {
		t.Fatal("substituted key must not verify against alice's peer id")
	}
}
