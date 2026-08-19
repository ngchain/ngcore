package consensus

// A "fake peer" is a bare libp2p host that speaks the wired protocol but
// replies with attacker-chosen messages (wrong message types, undecodable
// payloads, empty/short chains). Wiring a real local node to it drives the
// error and reject branches of every getRemote*/fetch* path that a
// well-behaved remote full node never triggers.

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	core "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	yamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-msgio"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

// fakePeer is a raw libp2p host answering the wired protocol with a
// caller-supplied reply builder.
type fakePeer struct {
	host       core.Host
	protocolID protocol.ID
	network    ngtypes.Network
}

// reply builds the message a fakePeer sends back for one request. It gets
// the fake peer (to sign/build headers) and the request's message ID so it
// can echo it (a mismatched ID is a separate failure path). Return nil to
// send nothing (hang up).
type reply func(fp *fakePeer, reqID []byte) *wired.Message

// newFakePeer boots a bare libp2p host on loopback and serves the wired
// protocol of the given network with the supplied reply builder.
func newFakePeer(t *testing.T, net ngtypes.Network, build reply) *fakePeer {
	t.Helper()

	h, err := libp2p.New(
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
	)
	if err != nil {
		t.Fatal(err)
	}

	fp := &fakePeer{
		host:       h,
		protocolID: protocol.ID(defaults.GetWiredProtocol(net)),
		network:    net,
	}

	h.SetStreamHandler(fp.protocolID, func(stream network.Stream) {
		defer stream.Close()

		r := msgio.NewReader(stream)
		raw, err := r.ReadMsg()
		if err != nil {
			return
		}
		var msg wired.Message
		if err := rlp.DecodeBytes(raw, &msg); err != nil {
			return
		}

		resp := build(fp, msg.Header.ID)
		if resp == nil {
			return
		}
		// sign so ReceiveReply's Verify passes; a caller that wants to
		// exercise the verify-failure path presets a (bogus) Sign itself
		if resp.Header.Sign == nil {
			sig, err := wired.Signature(fp.host, resp)
			if err != nil {
				return
			}
			resp.Header.Sign = sig
		}
		_ = wired.Reply(stream, resp)
	})

	t.Cleanup(func() { _ = h.Close() })

	return fp
}

// header builds a signed-ready wired header for this fake peer.
func (fp *fakePeer) header(reqID []byte, t wired.MsgType) *wired.MsgHeader {
	return wired.NewHeader(fp.host, fp.network, reqID, t)
}

// connectFake dials the fake peer from a real local node and waits until
// the wired protocol is announced, so the sync module treats it as a peer.
func connectFake(t *testing.T, local *PoWork, fp *fakePeer) {
	t.Helper()

	err := local.LocalNode.Connect(context.Background(), peer.AddrInfo{
		ID:    fp.host.ID(),
		Addrs: fp.host.Addrs(),
	})
	if err != nil {
		t.Fatalf("failed to dial the fake peer: %s", err)
	}

	waitUntil(t, 10*time.Second, func() bool {
		p, _ := local.LocalNode.Peerstore().FirstSupportedProtocol(
			fp.host.ID(), local.LocalNode.GetWiredProtocol())
		return p == local.LocalNode.GetWiredProtocol()
	}, "the fake peer never announced the wired protocol")
}

// recordForFake builds a RemoteRecord addressed at the fake peer's ID.
func recordForFake(fp *fakePeer, origin, latest uint64, cpHash, cpDiff []byte) *RemoteRecord {
	return NewRemoteRecord(fp.host.ID(), origin, latest, cpHash, cpDiff)
}
