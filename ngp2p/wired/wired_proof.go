package wired

import (
	"bytes"

	"github.com/c0mm4nd/rlp"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// ProofRequest selects one committed state leaf to prove. Domain is one of
// {balance,key,contract,commit}; Key is the domain's raw bucket key (address
// bytes for balance/key/contract, heightLE(8)‖Hash(32) for commit). Height 0
// means "at the current tip"; a non-zero Height asks for the proof against a
// specific historical state (best-effort — only archive/replayable nodes can
// serve it).
type ProofRequest struct {
	Domain string
	Key    []byte
	Height uint64
}

// ProofResponse is a self-contained state-inclusion (or absence) proof. It
// carries the Height/BlockHash/StateRoot it was generated against so the
// client can bind it to a header it independently trusts, then fold the
// branch back into StateRoot via statetrie.Verify. Found=false with an empty
// Value + zero ValueHash is a valid ABSENCE proof.
type ProofResponse struct {
	Height    uint64
	BlockHash []byte
	StateRoot []byte
	Domain    string
	Key       []byte
	Value     []byte
	ValueHash []byte
	Path      []byte
	Proof     [][]byte
	Found     bool
}

// wire-size caps for proof messages: a request is tiny (a short domain string
// plus a bucket key ≤ 40 bytes), a response carries a fixed 256-sibling branch
// (≈8KiB) plus the small committed value. An oversized frame is dropped before
// decoding, mirroring the stem path's defensive bounds.
const (
	maxProofReqWire  = 1 << 10  // 1 KiB: domain + key + height, generous slack
	maxProofRespWire = 64 << 10 // 64 KiB: 256×32 branch + value + envelope
)

// maxProofKeyWire caps the raw bucket key so a malformed request can never
// drive a huge LeafPath allocation. The widest legit key is the commit
// domain's heightLE(8)‖Hash(32) = 40 bytes.
const maxProofKeyWire = 64

// SendGetProof issues a state-proof request to one peer and returns the open
// stream so the caller reads the typed Proof reply (mirrors SendGetChain).
func (w *Wired) SendGetProof(peerID peer.ID, req ProofRequest) (id []byte, stream network.Stream, err error) {
	payload, err := rlp.EncodeToBytes(&req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to encode proof request")
	}

	id, _ = uuid.New().MarshalBinary()

	msg := &Message{
		Header:  NewHeader(w.host, w.network, id, GetProofMsg),
		Payload: payload,
	}

	signature, err := Signature(w.host, msg)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign proof request")
	}
	msg.Header.Sign = signature

	stream, err = Send(w.host, w.protocolID, peerID, msg)
	if err != nil {
		return nil, nil, err
	}

	log.Debugf("getproof to %s sent, message id: %x, domain=%s height=%d", peerID, id, req.Domain, req.Height)
	return id, stream, nil
}

// RequestProof is the one-shot client helper: it opens a wired stream to a
// single peer, sends GetProof, reads the signed Proof reply and decodes it.
// The returned ProofResponse is NOT yet trusted — the caller must bind it to
// a header it trusts via lightclient.VerifyProof.
func (w *Wired) RequestProof(peerID peer.ID, req ProofRequest) (*ProofResponse, error) {
	id, stream, err := w.SendGetProof(peerID, req)
	if err != nil {
		return nil, err
	}

	msg, err := ReceiveReply(id, stream)
	if err != nil {
		return nil, err
	}

	if msg.Header.Type == RejectMsg {
		return nil, errors.Errorf("peer rejected proof request: %s", msg.Payload)
	}
	if msg.Header.Type != ProofMsg {
		return nil, errors.Errorf("unexpected reply type %s to a proof request", msg.Header.Type)
	}
	// defensive cap on the reply: a well-formed proof is a fixed 256-sibling
	// branch plus a small value; refuse an oversized frame from a peer before
	// decoding it
	if len(msg.Payload) > maxProofRespWire {
		return nil, errors.Errorf("oversized proof reply (%d bytes)", len(msg.Payload))
	}

	return DecodeProofResponse(msg.Payload)
}

// DecodeProofResponse unmarshals a ProofMsg payload.
func DecodeProofResponse(rawPayload []byte) (*ProofResponse, error) {
	var resp ProofResponse
	if err := rlp.DecodeBytes(rawPayload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// sendProof signs and replies a ProofResponse.
func (w *Wired) sendProof(uuid []byte, stream network.Stream, resp *ProofResponse) bool {
	rawPayload, err := rlp.EncodeToBytes(resp)
	if err != nil {
		log.Debugf("failed to encode proof response: %s", err)
		return false
	}

	msg := &Message{
		Header:  NewHeader(w.host, w.network, uuid, ProofMsg),
		Payload: rawPayload,
	}

	signature, err := Signature(w.host, msg)
	if err != nil {
		log.Debugf("failed to sign proof response")
		return false
	}
	msg.Header.Sign = signature

	if err := Reply(stream, msg); err != nil {
		log.Debugf("failed sending proof to %s: %s", stream.Conn().RemotePeer(), err)
		return false
	}

	log.Debugf("sent proof to %s with message id: %x", stream.Conn().RemotePeer(), uuid)
	return true
}

// onGetProof serves a state proof. Defensive: oversized/malformed requests are
// rejected, never crash the handler (mirrors onGetChain). For Height==0 it
// builds the proof against the current tip in one consistent read txn (the
// block store and the state trie share the db, so the tip header's StateRoot
// and the proof root come from the same snapshot). For a specific Height it
// reconstructs that historical state (archive/replayable nodes only) and
// rejects cleanly otherwise.
func (w *Wired) onGetProof(stream network.Stream, msg *Message) {
	log.Debugf("received getproof request from %s", stream.Conn().RemotePeer())

	if len(msg.Payload) > maxProofReqWire {
		w.sendReject(msg.Header.ID, stream, errors.Errorf("getproof: oversized request (%d bytes)", len(msg.Payload)))
		return
	}

	var req ProofRequest
	if err := rlp.DecodeBytes(msg.Payload, &req); err != nil {
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	if len(req.Key) > maxProofKeyWire {
		w.sendReject(msg.Header.ID, stream, errors.Errorf("getproof: oversized key (%d bytes)", len(req.Key)))
		return
	}

	resp, err := w.buildProof(req)
	if err != nil {
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	w.sendProof(msg.Header.ID, stream, resp)
}

// buildProof produces a self-contained ProofResponse for the request. It
// reads the block store and the state trie inside a single read txn so the
// returned StateRoot matches the block header's committed StateRoot.
func (w *Wired) buildProof(req ProofRequest) (*ProofResponse, error) {
	tipHeight := w.chain.GetLatestBlockHeight()

	if req.Height == 0 || req.Height == tipHeight {
		return w.buildTipProof(req)
	}

	if req.Height > tipHeight {
		return nil, errors.Errorf("getproof: height %d is above the tip %d", req.Height, tipHeight)
	}

	return w.buildHistoricalProof(req)
}

// buildTipProof builds the proof against the current tip, reading the tip
// block header and the proof in one txn for a consistent snapshot.
func (w *Wired) buildTipProof(req ProofRequest) (*ProofResponse, error) {
	resp := &ProofResponse{
		Domain: req.Domain,
		Key:    append([]byte{}, req.Key...),
	}

	err := w.chain.State.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		block, err := ngblocks.GetLatestBlock(blockBucket)
		if err != nil {
			return err
		}

		root, path, value, valueHash, proof, err := ngstate.StateProof(txn, req.Domain, req.Key)
		if err != nil {
			return err
		}

		resp.Height = block.GetHeight()
		resp.BlockHash = block.GetHash()
		resp.StateRoot = block.BlockHeader.StateRoot
		resp.Path = path
		resp.Value = value
		resp.ValueHash = valueHash
		resp.Proof = proof
		resp.Found = len(value) != 0

		// bind to the header, not just the live trie: the block's committed
		// StateRoot IS the live root by the apply-time CheckStateRoot; assert
		// it here so a serving node never emits a proof against a header whose
		// root it cannot reproduce
		if !bytes.Equal(root, block.BlockHeader.StateRoot) {
			return errors.Errorf("getproof: tip live root %x != header StateRoot %x", root, block.BlockHeader.StateRoot)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// buildHistoricalProof reconstructs the full state as of req.Height in an
// isolated scratch db and proves the leaf there, binding to the historical
// block's header. Best-effort: only nodes that still have the blocks in range
// (or an archive) can serve it; a snapshot-synced node querying pre-checkpoint
// history errors cleanly. The block header binding comes from the live store.
func (w *Wired) buildHistoricalProof(req ProofRequest) (*ProofResponse, error) {
	block, err := w.chain.GetBlockByHeight(req.Height)
	if err != nil {
		return nil, errors.Wrapf(err, "getproof: chain lacks block@%d", req.Height)
	}
	header := block.(*ngtypes.FullBlock).BlockHeader

	resp := &ProofResponse{
		Height:    req.Height,
		BlockHash: block.GetHash(),
		StateRoot: header.StateRoot,
		Domain:    req.Domain,
		Key:       append([]byte{}, req.Key...),
	}

	err = w.chain.State.ReconstructAt(req.Height, func(txn *bbolt.Tx) error {
		root, path, value, valueHash, proof, e := ngstate.StateProof(txn, req.Domain, req.Key)
		if e != nil {
			return e
		}
		if !bytes.Equal(root, header.StateRoot) {
			return errors.Errorf("getproof: reconstructed root %x != header StateRoot %x at height %d", root, header.StateRoot, req.Height)
		}
		resp.Path = path
		resp.Value = value
		resp.ValueHash = valueHash
		resp.Proof = proof
		resp.Found = len(value) != 0
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "getproof: cannot reconstruct state@%d (archive required)", req.Height)
	}

	return resp, nil
}
