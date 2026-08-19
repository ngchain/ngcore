package wired

import (
	"context"
	"errors"
	"testing"
	"time"

	core "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-msgio"

	"github.com/ngchain/ngcore/ngtypes"
)

// ---------------------------------------------------------------------------
// failing-key host wrapper: everything delegates to a real host except the
// private key, whose Sign() always fails. This exercises the "failed to sign"
// branch of every send*/Send*-style helper without touching production code.
// ---------------------------------------------------------------------------

type failSignKey struct {
	crypto.PrivKey
}

func (k failSignKey) Sign([]byte) ([]byte, error) {
	return nil, errors.New("injected sign failure")
}

type failSignStore struct {
	peerstore.Peerstore
	id  core.PeerID
	key crypto.PrivKey
}

func (s failSignStore) PrivKey(p core.PeerID) crypto.PrivKey {
	if p == s.id {
		return s.key
	}
	return s.Peerstore.PrivKey(p)
}

type failSignHost struct {
	core.Host
}

func (h failSignHost) Peerstore() peerstore.Peerstore {
	real := h.Host.Peerstore()
	return failSignStore{
		Peerstore: real,
		id:        h.Host.ID(),
		key:       failSignKey{real.PrivKey(h.Host.ID())},
	}
}

// withFailingSign swaps the wired host for one whose key cannot sign, runs fn,
// then restores the original host.
func withFailingSign(w *Wired, fn func()) {
	orig := w.host
	w.host = failSignHost{orig}
	defer func() { w.host = orig }()
	fn()
}

// sinkProto is a protocol the client accepts so the server can open a stream
// to it. Handlers simply block, letting the test drive the stream lifecycle.
const sinkProto = "/wired-test/sink"

// serverToClientStream opens a server->client stream on a protocol the client
// accepts, so the server can Reply onto it.
func serverToClientStream(t *testing.T, fx *wiredFixture) network.Stream {
	t.Helper()

	fx.clientHost.SetStreamHandler(sinkProto, func(s network.Stream) {
		// drain and hold; the test controls the stream from the server side
		_, _ = msgio.NewReader(s).ReadMsg()
	})

	stream, err := fx.serverHost.NewStream(context.Background(), fx.clientHost.ID(), sinkProto)
	if err != nil {
		t.Fatal(err)
	}
	return stream
}

// resetStream opens a server->client stream and resets it, so any subsequent
// write from the server side fails.
func resetStream(t *testing.T, fx *wiredFixture) network.Stream {
	t.Helper()

	stream := serverToClientStream(t, fx)
	_ = stream.Reset()
	// give the reset a moment to propagate across the mocknet
	time.Sleep(50 * time.Millisecond)

	return stream
}

// ---------------------------------------------------------------------------
// client-side Send* signature-failure branches
// ---------------------------------------------------------------------------

func TestSendPingSignatureFailure(t *testing.T) {
	fx := newWiredFixture(t, 0)

	withFailingSign(fx.client, func() {
		id, stream := fx.client.SendPing(fx.serverHost.ID(), 0, 0, nil, nil)
		if id != nil || stream != nil {
			t.Fatal("SendPing must fail when signing fails")
		}
	})
}

func TestSendGetChainSignatureFailure(t *testing.T) {
	fx := newWiredFixture(t, 0)

	withFailingSign(fx.client, func() {
		if _, _, err := fx.client.SendGetChain(fx.serverHost.ID(), nil, nil); err == nil {
			t.Fatal("SendGetChain must fail when signing fails")
		}
	})
}

func TestSendGetSheetSignatureFailure(t *testing.T) {
	fx := newWiredFixture(t, 0)

	withFailingSign(fx.client, func() {
		if _, _, err := fx.client.SendGetSheet(fx.serverHost.ID(), 0, nil); err == nil {
			t.Fatal("SendGetSheet must fail when signing fails")
		}
	})
}

// ---------------------------------------------------------------------------
// server-side send* signature-failure branches (called directly on a live
// stream; the reply is never produced because signing fails)
// ---------------------------------------------------------------------------

func TestServerSendSignatureFailures(t *testing.T) {
	fx := newWiredFixture(t, 1)

	openStream := func() network.Stream {
		return serverToClientStream(t, fx)
	}

	withFailingSign(fx.server, func() {
		if fx.server.sendPong(nil, openStream(), 0, 0, nil, nil) {
			t.Error("sendPong must fail when signing fails")
		}
		if fx.server.sendReject(nil, openStream(), errors.New("boom")) {
			t.Error("sendReject must fail when signing fails")
		}
		if fx.server.sendSheet(nil, openStream(), &ngtypes.Sheet{}) {
			t.Error("sendSheet must fail when signing fails")
		}
		if fx.server.sendChain(nil, openStream(), fx.blocks[1]) {
			t.Error("sendChain must fail when signing fails")
		}
	})
}

// ---------------------------------------------------------------------------
// server-side send* write-failure branches (Reply write to a reset stream)
// ---------------------------------------------------------------------------

func TestServerSendReplyWriteFailures(t *testing.T) {
	fx := newWiredFixture(t, 1)

	if fx.server.sendPong(nil, resetStream(t, fx), 0, 0, nil, nil) {
		t.Error("sendPong must fail writing to a reset stream")
	}
	if fx.server.sendReject(nil, resetStream(t, fx), errors.New("boom")) {
		t.Error("sendReject must fail writing to a reset stream")
	}
	if fx.server.sendSheet(nil, resetStream(t, fx), &ngtypes.Sheet{}) {
		t.Error("sendSheet must fail writing to a reset stream")
	}
	if fx.server.sendChain(nil, resetStream(t, fx), fx.blocks[1]) {
		t.Error("sendChain must fail writing to a reset stream")
	}
}

// NOTE: ReceiveReply's `msg.Header == nil` branch (message_recv.go:39) is
// unreachable through the wire: the rlp codec cannot decode a struct with a
// nil pointer field — a frame carrying a nil Header fails earlier with
// "too few elements for wired.MsgHeader" from rlp.DecodeBytes. Likewise the
// post-read `stream.Close()` error branch (message_recv.go:29) cannot be hit
// with mocknet streams, which never fail on Close after a successful read.

// ---------------------------------------------------------------------------
// Verify / verifyMessageData error branches
// ---------------------------------------------------------------------------

// TestVerifyRejectsUnmarshalablePeerKey drives verifyMessageData's
// UnmarshalPublicKey error branch.
func TestVerifyRejectsUnmarshalablePeerKey(t *testing.T) {
	h := newTestHost(t)

	msg := signedMessage(t, h, []byte("hello"))
	msg.Header.PeerKey = []byte{0x00, 0x01, 0x02}
	if Verify(h.ID(), msg) {
		t.Fatal("an unmarshalable peer key must not verify")
	}
}
