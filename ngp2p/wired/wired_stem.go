package wired

import (
	"github.com/c0mm4nd/rlp"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// StemPayload wraps a Dandelion++ stem-phase item: the rlp bytes of a
// FullTx or Commitment plus the remaining hop budget. The TTL caps stem
// loops (successor graphs may contain cycles): each hop decrements it and
// fluffs at zero.
type StemPayload struct {
	TTL uint8
	Raw []byte
}

// wire-size caps for stem payloads, mirroring the pubsub validators'
// bounds (broadcast.maxTxWireSize / maxCommitWireSize) plus a little
// envelope slack: an oversized frame is dropped before decoding.
const (
	maxStemTxWire     = ngtypes.TxMaxExtraSize + 64<<10 + 1024
	maxStemCommitWire = 64<<10 + 1024
)

// sendStem signs and fire-and-forgets one stem message to a single peer.
// Each hop signs to its successor — a hop only ever learns its
// predecessor, which the direct TCP stream reveals anyway.
func (w *Wired) sendStem(peerID peer.ID, msgType MsgType, ttl uint8, raw []byte) error {
	payload, err := rlp.EncodeToBytes(&StemPayload{TTL: ttl, Raw: raw})
	if err != nil {
		return errors.Wrap(err, "failed to encode stem payload")
	}

	id, _ := uuid.New().MarshalBinary()

	req := &Message{
		Header:  NewHeader(w.host, w.network, id, msgType),
		Payload: payload,
	}

	signature, err := Signature(w.host, req)
	if err != nil {
		return errors.Wrap(err, "failed to sign stem message")
	}
	req.Header.Sign = signature

	stream, err := Send(w.host, w.protocolID, peerID, req)
	if err != nil {
		return err
	}

	log.Debugf("sent %s to %s, message id: %x", msgType, peerID, id)

	// no reply on the stem path: close our side right away
	return stream.Close()
}

// SendStemTx relays a tx to ONE peer over the wired stream (the
// Dandelion++ stem phase), instead of flooding it via pubsub.
func (w *Wired) SendStemTx(peerID peer.ID, ttl uint8, tx *ngtypes.FullTx) error {
	raw, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return errors.Wrap(err, "failed to encode stem tx")
	}

	return w.sendStem(peerID, StemTxMsg, ttl, raw)
}

// SendStemCommit relays a blind commitment to ONE peer over the wired
// stream (the Dandelion++ stem phase).
func (w *Wired) SendStemCommit(peerID peer.ID, ttl uint8, commit *ngtypes.Commitment) error {
	raw, err := rlp.EncodeToBytes(commit)
	if err != nil {
		return errors.Wrap(err, "failed to encode stem commitment")
	}

	return w.sendStem(peerID, StemCommitMsg, ttl, raw)
}

// decodeStemPayload applies the shared defensive gates: size cap and rlp
// shape. Malformed or oversized frames are dropped quietly — the stem
// path is fire-and-forget, so there is nobody to reply to, and a bad
// frame from ANY peer must never crash the handler.
func decodeStemPayload(msg *Message, maxWire int) (*StemPayload, bool) {
	if len(msg.Payload) > maxWire {
		log.Debugf("oversized stem payload (%d bytes), dropped", len(msg.Payload))
		return nil, false
	}

	var payload StemPayload
	if err := rlp.DecodeBytes(msg.Payload, &payload); err != nil {
		log.Debugf("malformed stem payload, dropped: %s", err)
		return nil, false
	}

	if len(payload.Raw) > maxWire {
		log.Debugf("oversized stem item (%d bytes), dropped", len(payload.Raw))
		return nil, false
	}

	return &payload, true
}

// onStemTx validates a stem-phase tx with the SAME stateless gates as the
// tx pubsub validator (network, envelope, signature; compact envelopes
// pass bounded for the pool to judge later, at fluff time) and hands it
// to the registered router. It never touches the pool: stem items enter a
// pool only when they eventually fluff through the normal pubsub path.
func (w *Wired) onStemTx(stream network.Stream, msg *Message) {
	if w.OnStemTx == nil {
		return // no router registered: drop, the origin's fail-safe re-fluffs
	}

	payload, ok := decodeStemPayload(msg, maxStemTxWire)
	if !ok {
		return
	}

	var tx ngtypes.FullTx
	if err := rlp.DecodeBytes(payload.Raw, &tx); err != nil {
		log.Debugf("malformed stem tx from %s, dropped: %s", stream.Conn().RemotePeer(), err)
		return
	}

	if tx.Network != w.network || !tx.IsSigned() {
		return
	}
	if !tx.IsCompactEnvelope() && tx.Verify(nil) != nil {
		return
	}

	w.OnStemTx(&tx, payload.TTL)
}

// onStemCommit is onStemTx for blind commitments, mirroring the commit
// pubsub validator's stateless gates.
func (w *Wired) onStemCommit(stream network.Stream, msg *Message) {
	if w.OnStemCommit == nil {
		return
	}

	payload, ok := decodeStemPayload(msg, maxStemCommitWire)
	if !ok {
		return
	}

	var commit ngtypes.Commitment
	if err := rlp.DecodeBytes(payload.Raw, &commit); err != nil {
		log.Debugf("malformed stem commitment from %s, dropped: %s", stream.Conn().RemotePeer(), err)
		return
	}

	if commit.Network != w.network || !commit.IsSigned() || len(commit.Hash) != ngtypes.HashSize {
		return
	}
	if commit.Sign[0] != 0x02 && commit.Verify(nil) != nil {
		// 0x02 = compact envelope, resolved against the on-chain key
		// registry by the pool at fluff time
		return
	}

	w.OnStemCommit(&commit, payload.TTL)
}
