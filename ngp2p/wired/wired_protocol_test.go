package wired

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/libp2p/go-msgio"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

func newTestChain(t *testing.T) *blockchain.Chain {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)

	return blockchain.Init(db, ngtypes.ZERONET, store, state)
}

// mineBlock builds and seals a valid ZERONET block on the parent, paying
// the block reward to the miner key (ZERONET's minimum difficulty seals
// within a few nonce attempts)
func mineBlock(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(), diff)

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}

	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}

	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			return block
		}
	}

	t.Fatal("failed to seal a ZERONET block within 1e6 nonces")
	return nil
}

type wiredFixture struct {
	serverHost host.Host
	clientHost host.Host

	server *Wired
	client *Wired

	chain *blockchain.Chain
	// blocks[i] is the block at height i (blocks[0] is the genesis)
	blocks []*ngtypes.FullBlock
}

// newWiredFixture spins up a two-node mocknet: one full wired server with
// a real chain of nBlocks mined blocks, plus a client. Fully hermetic
func newWiredFixture(t *testing.T, nBlocks int) *wiredFixture {
	t.Helper()

	chain := newTestChain(t)

	miner, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	blocks := []*ngtypes.FullBlock{chain.GetOriginBlock()}
	for i := 0; i < nBlocks; i++ {
		block := mineBlock(t, blocks[len(blocks)-1], miner)
		if err := chain.ApplyBlock(block); err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, block)
	}

	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}
	if err := mn.ConnectAllButSelf(); err != nil {
		t.Fatal(err)
	}

	server := NewWiredProtocol(serverHost, ngtypes.ZERONET, chain)
	server.GoServe()

	client := NewWiredProtocol(clientHost, ngtypes.ZERONET, chain)

	return &wiredFixture{
		serverHost: serverHost,
		clientHost: clientHost,
		server:     server,
		client:     client,
		chain:      chain,
		blocks:     blocks,
	}
}

func packHeights(from, to uint64) []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[0:8], from)
	binary.LittleEndian.PutUint64(buf[8:16], to)

	return buf
}

func mustReceive(t *testing.T, id []byte, stream network.Stream) *Message {
	t.Helper()

	_ = stream.SetDeadline(time.Now().Add(10 * time.Second))

	msg, err := ReceiveReply(id, stream)
	if err != nil {
		t.Fatal(err)
	}

	return msg
}

func chainBlocks(t *testing.T, msg *Message) []*ngtypes.FullBlock {
	t.Helper()

	if msg.Header.Type != ChainMsg {
		t.Fatalf("expected ChainMsg, got %s (payload %q)", msg.Header.Type, msg.Payload)
	}

	payload, err := DecodeChainPayload(msg.Payload)
	if err != nil {
		t.Fatal(err)
	}

	return payload.Blocks
}

func randomPeerID(t *testing.T) peer.ID {
	t.Helper()

	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		t.Fatal(err)
	}

	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	return id
}

// sendSigned crafts, signs and sends a raw wired message from the client,
// optionally tampering it after signing
func sendSigned(t *testing.T, fx *wiredFixture, msgType MsgType, payload []byte, tamper bool) (id []byte, stream network.Stream) {
	t.Helper()

	id, err := uuid.New().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	req := &Message{
		Header:  NewHeader(fx.clientHost, ngtypes.ZERONET, id, msgType),
		Payload: payload,
	}

	sign, err := Signature(fx.clientHost, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Sign = sign

	if tamper {
		req.Header.Timestamp++
	}

	stream, err = Send(fx.clientHost, fx.client.protocolID, fx.serverHost.ID(), req)
	if err != nil {
		t.Fatal(err)
	}

	return id, stream
}

// ---------------------------------------------------------------------------
// plain unit tests
// ---------------------------------------------------------------------------

func TestMsgTypeString(t *testing.T) {
	cases := map[MsgType]string{
		InvalidMsg:    "InvalidMsg",
		PingMsg:       "PingMsg",
		PongMsg:       "PongMsg",
		RejectMsg:     "RejectMsg",
		GetChainMsg:   fmt.Sprintf("UnknownMsg: %d", uint8(GetChainMsg)),
		MsgType(0xee): "UnknownMsg: 238",
	}

	for mt, want := range cases {
		if got := mt.String(); got != want {
			t.Errorf("MsgType(%d).String() = %q, want %q", mt, got, want)
		}
	}
}

func TestNewHeaderFields(t *testing.T) {
	h := newTestHost(t)

	id := []byte("some-id-16-bytes")
	header := NewHeader(h, ngtypes.ZERONET, id, PingMsg)

	if header.Network != ngtypes.ZERONET {
		t.Errorf("wrong network: %v", header.Network)
	}
	if !bytes.Equal(header.ID, id) {
		t.Errorf("wrong id: %x", header.ID)
	}
	if header.Type != PingMsg {
		t.Errorf("wrong type: %s", header.Type)
	}
	if header.Timestamp == 0 {
		t.Error("timestamp not set")
	}
	if header.Sign != nil {
		t.Error("fresh header must be unsigned")
	}

	// the embedded key must round-trip to the host's peer id
	pub, err := crypto.UnmarshalPublicKey(header.PeerKey)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != h.ID() {
		t.Error("header key does not match the host identity")
	}
}

func TestDecodePayloadErrors(t *testing.T) {
	garbage := []byte{0xff}

	if _, err := DecodePongPayload(garbage); err == nil {
		t.Error("DecodePongPayload must fail on garbage")
	}
	if _, err := DecodeChainPayload(garbage); err == nil {
		t.Error("DecodeChainPayload must fail on garbage")
	}
	if _, err := DecodeSheetPayload(garbage); err == nil {
		t.Error("DecodeSheetPayload must fail on garbage")
	}
}

func TestSendErrors(t *testing.T) {
	fx := newWiredFixture(t, 0)

	// un-encodable data
	if _, err := Send(fx.clientHost, fx.client.protocolID, fx.serverHost.ID(), make(chan int)); err == nil {
		t.Error("Send must fail on un-encodable data")
	}

	// unknown peer: no route on the mocknet
	if _, err := Send(fx.clientHost, fx.client.protocolID, randomPeerID(t), "data"); err == nil {
		t.Error("Send must fail for an unknown peer")
	}
}

func TestReplyErrors(t *testing.T) {
	fx := newWiredFixture(t, 0)

	stream, err := fx.clientHost.NewStream(context.Background(), fx.serverHost.ID(), fx.client.protocolID)
	if err != nil {
		t.Fatal(err)
	}

	// un-encodable data fails before any write
	if err := Reply(stream, make(chan int)); err == nil {
		t.Error("Reply must fail on un-encodable data")
	}

	// a reset stream fails on write
	_ = stream.Reset()
	if err := Reply(stream, "data"); err == nil {
		t.Error("Reply must fail on a reset stream")
	}
}

// ---------------------------------------------------------------------------
// ReceiveReply behaviors (custom raw responders)
// ---------------------------------------------------------------------------

// replyRaw registers a responder protocol on the server host and opens a
// client stream to it
func replyRaw(t *testing.T, fx *wiredFixture, pid protocol.ID, respond func(network.Stream)) network.Stream {
	t.Helper()

	fx.serverHost.SetStreamHandler(pid, func(s network.Stream) {
		respond(s)
	})

	stream, err := fx.clientHost.NewStream(context.Background(), fx.serverHost.ID(), pid)
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.SetDeadline(time.Now().Add(10 * time.Second))

	return stream
}

func serverReply(t *testing.T, fx *wiredFixture, msgType MsgType, id []byte, signed bool) func(network.Stream) {
	t.Helper()

	return func(s network.Stream) {
		msg := &Message{
			Header:  NewHeader(fx.serverHost, ngtypes.ZERONET, id, msgType),
			Payload: []byte("payload"),
		}

		if signed {
			sign, err := Signature(fx.serverHost, msg)
			if err != nil {
				t.Error(err)
				return
			}
			msg.Header.Sign = sign
		}

		if err := Reply(s, msg); err != nil {
			t.Error(err)
		}
	}
}

func TestReceiveReplyErrors(t *testing.T) {
	fx := newWiredFixture(t, 0)
	reqID := []byte("0123456789abcdef")

	t.Run("garbage reply", func(t *testing.T) {
		stream := replyRaw(t, fx, "/wired-test/garbage", func(s network.Stream) {
			_ = msgio.NewWriter(s).WriteMsg([]byte{0xff})
		})
		if _, err := ReceiveReply(reqID, stream); err == nil {
			t.Error("garbage reply must not decode")
		}
	})

	t.Run("closed without reply", func(t *testing.T) {
		stream := replyRaw(t, fx, "/wired-test/eof", func(s network.Stream) {
			_ = s.Close()
		})
		if _, err := ReceiveReply(reqID, stream); err == nil {
			t.Error("an EOF stream must error")
		}
	})

	t.Run("invalid msg type", func(t *testing.T) {
		stream := replyRaw(t, fx, "/wired-test/invalidtype", serverReply(t, fx, InvalidMsg, reqID, true))
		_, err := ReceiveReply(reqID, stream)
		if !errors.Is(err, ErrMsgTypeInvalid) {
			t.Errorf("want ErrMsgTypeInvalid, got %v", err)
		}
	})

	t.Run("wrong id", func(t *testing.T) {
		stream := replyRaw(t, fx, "/wired-test/wrongid", serverReply(t, fx, PongMsg, []byte("fedcba9876543210"), true))
		_, err := ReceiveReply(reqID, stream)
		if !errors.Is(err, ErrMsgIDInvalid) {
			t.Errorf("want ErrMsgIDInvalid, got %v", err)
		}
	})

	t.Run("unsigned reply", func(t *testing.T) {
		stream := replyRaw(t, fx, "/wired-test/unsigned", serverReply(t, fx, PongMsg, reqID, false))
		_, err := ReceiveReply(reqID, stream)
		if !errors.Is(err, ErrMsgSignInvalid) {
			t.Errorf("want ErrMsgSignInvalid, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// ping / pong
// ---------------------------------------------------------------------------

func TestWiredPingPong(t *testing.T) {
	fx := newWiredFixture(t, 3)

	if fx.server.GetWiredProtocol() != fx.client.GetWiredProtocol() {
		t.Fatal("both peers must speak the same protocol")
	}

	id, stream := fx.client.SendPing(fx.serverHost.ID(), 0, 3,
		fx.blocks[0].GetHash(), fx.blocks[0].GetActualDiff().Bytes())
	if id == nil || stream == nil {
		t.Fatal("SendPing failed")
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != PongMsg {
		t.Fatalf("expected PongMsg, got %s (payload %q)", msg.Header.Type, msg.Payload)
	}

	pong, err := DecodePongPayload(msg.Payload)
	if err != nil {
		t.Fatal(err)
	}

	if pong.Origin != 0 {
		t.Errorf("pong origin = %d, want 0", pong.Origin)
	}
	if pong.Latest != 3 {
		t.Errorf("pong latest = %d, want 3", pong.Latest)
	}
	// with 3 blocks the latest checkpoint is still the genesis
	if !bytes.Equal(pong.CheckpointHash, fx.blocks[0].GetHash()) {
		t.Errorf("pong checkpoint hash mismatch: %x", pong.CheckpointHash)
	}
}

func TestSendPingUnknownPeer(t *testing.T) {
	fx := newWiredFixture(t, 0)

	id, stream := fx.client.SendPing(randomPeerID(t), 0, 0, nil, nil)
	if id != nil || stream != nil {
		t.Fatal("SendPing to an unknown peer must fail")
	}
}

func TestWiredRejectsBadPingPayload(t *testing.T) {
	fx := newWiredFixture(t, 0)

	id, stream := sendSigned(t, fx, PingMsg, []byte{0xff}, false)

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

// ---------------------------------------------------------------------------
// reject paths
// ---------------------------------------------------------------------------

func TestWiredRejectsUnknownType(t *testing.T) {
	fx := newWiredFixture(t, 0)

	// a response-only type is not servable
	id, stream := sendSigned(t, fx, SheetMsg, nil, false)

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
	if !bytes.Contains(msg.Payload, []byte(ErrMsgTypeInvalid.Error())) {
		t.Errorf("unexpected reject reason: %q", msg.Payload)
	}
}

func TestWiredRejectsTamperedSign(t *testing.T) {
	fx := newWiredFixture(t, 0)

	id, stream := sendSigned(t, fx, PingMsg, nil, true)

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
	if !bytes.Contains(msg.Payload, []byte(ErrMsgSignInvalid.Error())) {
		t.Errorf("unexpected reject reason: %q", msg.Payload)
	}
}

func TestHandleStreamMalformed(t *testing.T) {
	fx := newWiredFixture(t, 0)

	// a frame which is not rlp at all
	s1, err := fx.clientHost.NewStream(context.Background(), fx.serverHost.ID(), fx.client.protocolID)
	if err != nil {
		t.Fatal(err)
	}
	if err := msgio.NewWriter(s1).WriteMsg([]byte{0xff}); err != nil {
		t.Fatal(err)
	}

	// a stream closed before any frame
	s2, err := fx.clientHost.NewStream(context.Background(), fx.serverHost.ID(), fx.client.protocolID)
	if err != nil {
		t.Fatal(err)
	}
	_ = s2.Close()

	// the server must survive both (no panic, no reply)
	time.Sleep(200 * time.Millisecond)
	_ = s1.Reset()

	// and still serve properly afterwards
	id, stream := fx.client.SendPing(fx.serverHost.ID(), 0, 0,
		fx.blocks[0].GetHash(), fx.blocks[0].GetActualDiff().Bytes())
	if id == nil || stream == nil {
		t.Fatal("SendPing failed after malformed streams")
	}
	if msg := mustReceive(t, id, stream); msg.Header.Type != PongMsg {
		t.Fatalf("expected PongMsg, got %s", msg.Header.Type)
	}
}

// ---------------------------------------------------------------------------
// getchain
// ---------------------------------------------------------------------------

func TestGetChainFetchByHeights(t *testing.T) {
	fx := newWiredFixture(t, 3)

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), nil, packHeights(1, 3))
	if err != nil {
		t.Fatal(err)
	}

	blocks := chainBlocks(t, mustReceive(t, id, stream))
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(blocks))
	}
	for i, block := range blocks {
		want := fx.blocks[i+1]
		if block.GetHeight() != want.GetHeight() || !bytes.Equal(block.GetHash(), want.GetHash()) {
			t.Errorf("block %d mismatch: got block@%d %x", i, block.GetHeight(), block.GetHash())
		}
	}
}

func TestGetChainFetchByHeightsMissing(t *testing.T) {
	fx := newWiredFixture(t, 2)

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), nil, packHeights(2, 9))
	if err != nil {
		t.Fatal(err)
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

func TestGetChainConvergeWithSamepoint(t *testing.T) {
	fx := newWiredFixture(t, 3)

	// block1 is known to the server, the second hash is not: the server
	// walks 1 block up from the samepoint
	from := [][]byte{fx.blocks[1].GetHash(), bytes.Repeat([]byte{0xde}, 32)}

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, packHeights(1, 2))
	if err != nil {
		t.Fatal(err)
	}

	blocks := chainBlocks(t, mustReceive(t, id, stream))
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if blocks[0].GetHeight() != 2 {
		t.Errorf("got block@%d, want block@2", blocks[0].GetHeight())
	}
}

func TestGetChainConvergeNoSamepoint(t *testing.T) {
	fx := newWiredFixture(t, 2)

	from := [][]byte{bytes.Repeat([]byte{0xde}, 32)}

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, packHeights(0, 1))
	if err != nil {
		t.Fatal(err)
	}

	blocks := chainBlocks(t, mustReceive(t, id, stream))
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].GetHeight() != 0 || blocks[1].GetHeight() != 1 {
		t.Errorf("got blocks @%d @%d, want @0 @1", blocks[0].GetHeight(), blocks[1].GetHeight())
	}
}

func TestGetChainConvergeNoSamepointMissing(t *testing.T) {
	fx := newWiredFixture(t, 2)

	from := [][]byte{bytes.Repeat([]byte{0xde}, 32)}

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, packHeights(5, 6))
	if err != nil {
		t.Fatal(err)
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

func TestGetChainConvergeMissingNext(t *testing.T) {
	fx := newWiredFixture(t, 3)

	// samepoint is the tip: the walk immediately runs off the chain
	from := [][]byte{fx.blocks[3].GetHash(), bytes.Repeat([]byte{0xde}, 32)}

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, packHeights(0, 0))
	if err != nil {
		t.Fatal(err)
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

func TestGetChainFetchByHash(t *testing.T) {
	fx := newWiredFixture(t, 3)

	from := [][]byte{fx.blocks[1].GetHash()}

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, fx.blocks[3].GetHash())
	if err != nil {
		t.Fatal(err)
	}

	blocks := chainBlocks(t, mustReceive(t, id, stream))
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].GetHeight() != 2 || blocks[1].GetHeight() != 3 {
		t.Errorf("got blocks @%d @%d, want @2 @3", blocks[0].GetHeight(), blocks[1].GetHeight())
	}
}

func TestGetChainFetchByHashUnknownFrom(t *testing.T) {
	fx := newWiredFixture(t, 2)

	from := [][]byte{bytes.Repeat([]byte{0xde}, 32)}

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, fx.blocks[2].GetHash())
	if err != nil {
		t.Fatal(err)
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

func TestGetChainFetchByHashUnreachableTo(t *testing.T) {
	fx := newWiredFixture(t, 3)

	from := [][]byte{fx.blocks[1].GetHash()}

	// To is unknown: the server walks to its tip and replies what it has
	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, bytes.Repeat([]byte{0xad}, 32))
	if err != nil {
		t.Fatal(err)
	}

	blocks := chainBlocks(t, mustReceive(t, id, stream))
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (heights 2..3)", len(blocks))
	}
}

// TestGetChainConvergeEmptyReplyClosesStream documents today's behavior
// for a converging request whose samepoint is the last known hash: the
// server has nothing to send, sendChain refuses empty chains and the
// stream is closed with NO reply at all (see the suspected-bug report:
// the requester cannot distinguish this from a failure)
func TestGetChainConvergeEmptyReplyClosesStream(t *testing.T) {
	fx := newWiredFixture(t, 2)

	from := [][]byte{fx.blocks[2].GetHash()} // the tip itself, nothing above it

	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), from, packHeights(0, 0))
	if err != nil {
		t.Fatal(err)
	}

	_ = stream.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := ReceiveReply(id, stream); err == nil {
		t.Fatal("expected an error (bare stream close), got a reply")
	}
}

func TestGetChainBadPayload(t *testing.T) {
	fx := newWiredFixture(t, 0)

	id, stream := sendSigned(t, fx, GetChainMsg, []byte{0xff}, false)

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

func TestSendGetChainUnknownPeer(t *testing.T) {
	fx := newWiredFixture(t, 0)

	// nil `to` also exercises the empty-hash substitution
	if _, _, err := fx.client.SendGetChain(randomPeerID(t), nil, nil); err == nil {
		t.Fatal("SendGetChain to an unknown peer must fail")
	}
}

func TestVerifyRejectsGarbagePeerKey(t *testing.T) {
	h := newTestHost(t)

	msg := signedMessage(t, h, []byte("hello"))
	msg.Header.PeerKey = []byte{0xde, 0xad}
	if Verify(h.ID(), msg) {
		t.Fatal("a message with an unparsable peer key must not verify")
	}
}

// ---------------------------------------------------------------------------
// getsheet
// ---------------------------------------------------------------------------

func TestGetSheet(t *testing.T) {
	fx := newWiredFixture(t, int(ngtypes.BlockCheckRound)) // reach the first checkpoint

	checkpoint := fx.blocks[ngtypes.BlockCheckRound]

	id, stream, err := fx.client.SendGetSheet(fx.serverHost.ID(), checkpoint.GetHeight(), checkpoint.GetHash())
	if err != nil {
		t.Fatal(err)
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != SheetMsg {
		t.Fatalf("expected SheetMsg, got %s (payload %q)", msg.Header.Type, msg.Payload)
	}

	payload, err := DecodeSheetPayload(msg.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Sheet == nil {
		t.Fatal("nil sheet in payload")
	}
	if payload.Sheet.Height != checkpoint.GetHeight() {
		t.Errorf("sheet height = %d, want %d", payload.Sheet.Height, checkpoint.GetHeight())
	}
	if !bytes.Equal(payload.Sheet.BlockHash, checkpoint.GetHash()) {
		t.Errorf("sheet block hash mismatch: %x", payload.Sheet.BlockHash)
	}
}

func TestGetSheetRejectsUnknownSnapshot(t *testing.T) {
	fx := newWiredFixture(t, 3)

	id, stream, err := fx.client.SendGetSheet(fx.serverHost.ID(), 3, bytes.Repeat([]byte{0xde}, 32))
	if err != nil {
		t.Fatal(err)
	}

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
	if !bytes.Contains(msg.Payload, []byte(ngstate.ErrSnapshotNofFound.Error())) {
		t.Errorf("unexpected reject reason: %q", msg.Payload)
	}
}

func TestGetSheetBadPayload(t *testing.T) {
	fx := newWiredFixture(t, 0)

	id, stream := sendSigned(t, fx, GetSheetMsg, []byte{0xff}, false)

	msg := mustReceive(t, id, stream)
	if msg.Header.Type != RejectMsg {
		t.Fatalf("expected RejectMsg, got %s", msg.Header.Type)
	}
}

func TestSendGetSheetUnknownPeer(t *testing.T) {
	fx := newWiredFixture(t, 0)

	if _, _, err := fx.client.SendGetSheet(randomPeerID(t), 0, nil); err == nil {
		t.Fatal("SendGetSheet to an unknown peer must fail")
	}
}

// TestGetChainMalformedRejectsNoPanic is the regression for the remote
// crash DoS: a payload with an EMPTY (non-nil) From and a 32-byte To used
// to fall through to From[0] and panic the server's stream handler. Every
// malformed shape must now come back as a reject, server still alive.
func TestGetChainMalformedRejectsNoPanic(t *testing.T) {
	fx := newWiredFixture(t, 3)

	cases := []struct {
		name string
		from [][]byte
		to   []byte
	}{
		{"empty From, 32-byte To", [][]byte{}, make([]byte, 32)}, // the crasher
		{"empty From, short To", [][]byte{}, make([]byte, 8)},    // To[0:8] OOB
		{"empty From, odd To", [][]byte{}, make([]byte, 20)},     // neither 16 nor 32
		{"nonempty From, odd To", [][]byte{make([]byte, 32)}, make([]byte, 7)},
	}
	for _, c := range cases {
		id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), c.from, c.to)
		if err != nil {
			t.Fatalf("%s: SendGetChain: %v", c.name, err)
		}
		reply, err := ReceiveReply(id, stream)
		if err != nil {
			t.Fatalf("%s: no reply (server likely crashed): %v", c.name, err)
		}
		if reply.Header.Type != RejectMsg {
			t.Fatalf("%s: got %s, want RejectMsg", c.name, reply.Header.Type)
		}
	}

	// the server is still serving after all the malformed requests
	id, stream, err := fx.client.SendGetChain(fx.serverHost.ID(), nil, packHeights(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveReply(id, stream); err != nil {
		t.Fatalf("server unresponsive after malformed requests: %v", err)
	}
}
