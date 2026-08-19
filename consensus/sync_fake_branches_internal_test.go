package consensus

// These tests point a real local node at a fakePeer that returns
// attacker-chosen wired replies, covering the reject / invalid-message-type
// / decode-failure branches of every getRemote*/fetch* path.

import (
	"errors"
	"testing"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestGetRemoteStatusFakeReplies(t *testing.T) {
	t.Run("reject", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{
				Header:  fp.header(reqID, wired.RejectMsg),
				Payload: []byte("nope"),
			}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		err := local.SyncMod.getRemoteStatus(fp.host.ID())
		if !errors.Is(err, ErrMsgRejected) {
			t.Fatalf("getRemoteStatus reject = %v, want ErrMsgRejected", err)
		}
	})

	t.Run("invalid message type", func(t *testing.T) {
		// a ChainMsg where a Pong is expected hits the default branch
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{
				Header:  fp.header(reqID, wired.ChainMsg),
				Payload: mustRLP(t, &wired.ChainPayload{}),
			}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		err := local.SyncMod.getRemoteStatus(fp.host.ID())
		if !errors.Is(err, ErrInvalidMsgType) {
			t.Fatalf("getRemoteStatus invalid type = %v, want ErrInvalidMsgType", err)
		}
	})

	t.Run("undecodable pong payload", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{
				Header:  fp.header(reqID, wired.PongMsg),
				Payload: []byte{0xff, 0xff, 0xff}, // not a valid rlp StatusPayload
			}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if err := local.SyncMod.getRemoteStatus(fp.host.ID()); err == nil {
			t.Fatal("getRemoteStatus with an undecodable pong must fail")
		}
	})

	t.Run("reply id mismatch", func(t *testing.T) {
		// echoing a wrong id makes ReceiveReply fail before the type switch
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{
				Header:  fp.header([]byte("not-the-request-id"), wired.PongMsg),
				Payload: mustRLP(t, &wired.StatusPayload{}),
			}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if err := local.SyncMod.getRemoteStatus(fp.host.ID()); err == nil {
			t.Fatal("getRemoteStatus with a mismatched reply id must fail")
		}
	})
}

func TestGetRemoteChainFakeReplies(t *testing.T) {
	rec := func(fp *fakePeer) *RemoteRecord {
		return recordForFake(fp, 0, 12, make([]byte, 32), []byte{0x01})
	}

	t.Run("reject", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.RejectMsg), Payload: []byte("no")}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChain(fp.host.ID(), [][]byte{{0x01}}, nil); !errors.Is(err, ErrMsgRejected) {
			t.Fatalf("getRemoteChain reject = %v, want ErrMsgRejected", err)
		}
	})

	t.Run("invalid message type", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.PongMsg), Payload: mustRLP(t, &wired.StatusPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChain(fp.host.ID(), [][]byte{{0x01}}, nil); !errors.Is(err, ErrInvalidMsgType) {
			t.Fatalf("getRemoteChain invalid type = %v, want ErrInvalidMsgType", err)
		}
	})

	t.Run("undecodable chain payload", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.ChainMsg), Payload: []byte{0xff, 0xff}}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChain(fp.host.ID(), [][]byte{{0x01}}, nil); err == nil {
			t.Fatal("getRemoteChain with an undecodable chain payload must fail")
		}
	})

	// a mismatched reply id makes ReceiveReply fail before the type switch
	t.Run("receive reply error", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header([]byte("wrong-id"), wired.ChainMsg), Payload: mustRLP(t, &wired.ChainPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChain(fp.host.ID(), [][]byte{{0x01}}, nil); err == nil {
			t.Fatal("getRemoteChain with a mismatched reply id must fail")
		}
	})

	// fetchRemoteRange: a ChainMsg with zero blocks trips its "no blocks"
	// guard
	t.Run("fetchRemoteRange empty round", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.ChainMsg), Payload: mustRLP(t, &wired.ChainPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.fetchRemoteRange(rec(fp), 1, 5); err == nil {
			t.Fatal("fetchRemoteRange over an empty round must fail")
		}
	})

	// fetchRemoteRange: propagates a getRemoteChain error
	t.Run("fetchRemoteRange reject", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.RejectMsg), Payload: []byte("no")}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.fetchRemoteRange(rec(fp), 1, 5); !errors.Is(err, ErrMsgRejected) {
			t.Fatalf("fetchRemoteRange reject = %v, want ErrMsgRejected", err)
		}
	})
}

func TestGetRemoteChainFromLocalLatestFakeReplies(t *testing.T) {
	rec := func(fp *fakePeer) *RemoteRecord {
		return recordForFake(fp, 0, 12, make([]byte, 32), []byte{0x01})
	}

	t.Run("reject", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.RejectMsg), Payload: []byte("no")}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChainFromLocalLatest(rec(fp)); !errors.Is(err, ErrMsgRejected) {
			t.Fatalf("getRemoteChainFromLocalLatest reject = %v, want ErrMsgRejected", err)
		}
	})

	t.Run("invalid message type", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.PongMsg), Payload: mustRLP(t, &wired.StatusPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChainFromLocalLatest(rec(fp)); !errors.Is(err, ErrInvalidMsgType) {
			t.Fatalf("getRemoteChainFromLocalLatest invalid type = %v, want ErrInvalidMsgType", err)
		}
	})

	t.Run("undecodable payload", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.ChainMsg), Payload: []byte{0xff, 0xff}}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChainFromLocalLatest(rec(fp)); err == nil {
			t.Fatal("getRemoteChainFromLocalLatest with an undecodable payload must fail")
		}
	})

	t.Run("receive reply error", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header([]byte("wrong-id"), wired.ChainMsg), Payload: mustRLP(t, &wired.ChainPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteChainFromLocalLatest(rec(fp)); err == nil {
			t.Fatal("getRemoteChainFromLocalLatest with a mismatched reply id must fail")
		}
	})

	t.Run("happy path returns blocks", func(t *testing.T) {
		// a valid (empty) ChainMsg exercises the success return
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.ChainMsg), Payload: mustRLP(t, &wired.ChainPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		blocks, err := local.SyncMod.getRemoteChainFromLocalLatest(rec(fp))
		if err != nil {
			t.Fatalf("getRemoteChainFromLocalLatest happy path failed: %s", err)
		}
		if len(blocks) != 0 {
			t.Fatalf("expected no blocks, got %d", len(blocks))
		}
	})
}

func TestGetRemoteStateSheetFakeReplies(t *testing.T) {
	rec := func(fp *fakePeer) *RemoteRecord {
		return recordForFake(fp, 0, 12, make([]byte, 32), []byte{0x01})
	}

	t.Run("invalid message type", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.PongMsg), Payload: mustRLP(t, &wired.StatusPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteStateSheet(rec(fp)); !errors.Is(err, ErrInvalidMsgType) {
			t.Fatalf("getRemoteStateSheet invalid type = %v, want ErrInvalidMsgType", err)
		}
	})

	t.Run("undecodable sheet payload", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.SheetMsg), Payload: []byte{0xff, 0xff}}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteStateSheet(rec(fp)); err == nil {
			t.Fatal("getRemoteStateSheet with an undecodable sheet payload must fail")
		}
	})

	t.Run("receive reply error", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header([]byte("wrong-id"), wired.SheetMsg), Payload: mustRLP(t, &wired.SheetPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteStateSheet(rec(fp)); err == nil {
			t.Fatal("getRemoteStateSheet with a mismatched reply id must fail")
		}
	})
}

func TestGetRemoteCheckpointFakeReplies(t *testing.T) {
	// checkpointHeight must exceed 2*BlockCheckRound so getRemoteCheckpoint
	// actually sends a request instead of returning genesis
	far := 4 * uint64(ngtypes.BlockCheckRound)
	rec := func(fp *fakePeer) *RemoteRecord {
		return recordForFake(fp, 0, far, make([]byte, 32), []byte{0x01})
	}

	t.Run("reject", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.RejectMsg), Payload: []byte("no")}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteCheckpoint(rec(fp)); !errors.Is(err, ErrMsgRejected) {
			t.Fatalf("getRemoteCheckpoint reject = %v, want ErrMsgRejected", err)
		}
	})

	t.Run("invalid message type", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.PongMsg), Payload: mustRLP(t, &wired.StatusPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteCheckpoint(rec(fp)); !errors.Is(err, wired.ErrMsgTypeInvalid) {
			t.Fatalf("getRemoteCheckpoint invalid type = %v, want ErrMsgTypeInvalid", err)
		}
	})

	t.Run("undecodable chain payload", func(t *testing.T) {
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.ChainMsg), Payload: []byte{0xff, 0xff}}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteCheckpoint(rec(fp)); err == nil {
			t.Fatal("getRemoteCheckpoint with an undecodable chain payload must fail")
		}
	})

	t.Run("wrong block count", func(t *testing.T) {
		// a ChainMsg with zero blocks hits the "should be 1" guard
		fp := newFakePeer(t, ngtypes.ZERONET, func(fp *fakePeer, reqID []byte) *wired.Message {
			return &wired.Message{Header: fp.header(reqID, wired.ChainMsg), Payload: mustRLP(t, &wired.ChainPayload{})}
		})
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		connectFake(t, local, fp)

		if _, err := local.SyncMod.getRemoteCheckpoint(rec(fp)); !errors.Is(err, wired.ErrMsgPayloadInvalid) {
			t.Fatalf("getRemoteCheckpoint wrong count = %v, want ErrMsgPayloadInvalid", err)
		}
	})

	t.Run("send error to unreachable peer", func(t *testing.T) {
		// SendGetChain itself fails when the peer is unknown
		local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
		bad := NewRemoteRecord(bogusPeer, 0, far, make([]byte, 32), []byte{0x01})
		if _, err := local.SyncMod.getRemoteCheckpoint(bad); err == nil {
			t.Fatal("getRemoteCheckpoint to an unreachable peer must fail")
		}
	})
}

func mustRLP(t *testing.T, v interface{}) []byte {
	t.Helper()
	raw, err := rlp.EncodeToBytes(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
