// Package lightclient verifies consensus-state Merkle proofs against a block
// header a light node already trusts. It closes the light-client loop: a full
// node serves a self-contained state proof over the wired protocol
// (ngp2p/wired GetProof/Proof), and the light node folds it back into the
// header's StateRoot here — a trustless state query with no RPC trust.
//
// It imports only statetrie + ngtypes (+ the wired response type), so a mobile
// wallet or a bridge can link it without the full node stack.
package lightclient

import (
	"bytes"

	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
)

// VerifyProof checks a wired ProofResponse against a header the caller already
// trusts and returns the proven value. It binds the proof to the header, then
// folds the Merkle branch into the committed root:
//
//  1. resp.StateRoot must equal header.StateRoot and resp.BlockHash must equal
//     the header's own hash — this pins the proof to THIS trusted header, so a
//     peer cannot answer with a proof against some other (attacker-chosen)
//     state.
//  2. statetrie.Verify folds the 256-sibling branch back into resp.StateRoot.
//  3. For an inclusion proof (Found), the leaf valueHash must be the hash of
//     the returned value, so the value is bound to the leaf. An absence proof
//     (Found=false, empty value, zero valueHash) is accepted too — statetrie
//     verifies a zero valueHash as proof of ABSENCE.
//
// On success it returns (value, true); value is nil for a verified absence.
func VerifyProof(header *ngtypes.BlockHeader, resp *wired.ProofResponse) (value []byte, ok bool) {
	if header == nil || resp == nil {
		return nil, false
	}

	// (1) bind the proof to the trusted header
	if !bytes.Equal(resp.StateRoot, header.StateRoot) {
		return nil, false
	}
	if !bytes.Equal(resp.BlockHash, header.GetHash()) {
		return nil, false
	}

	// (3a) the value must hash to the claimed leaf valueHash. For absence the
	// value is empty and the valueHash is the zero hash, matching the trie's
	// per-domain "empty value == absent leaf" encoding.
	var wantValueHash []byte
	if resp.Found {
		if len(resp.Value) == 0 {
			return nil, false // Found must carry a value
		}
		wantValueHash = statetrie.ValueHash(resp.Value)
	} else {
		if len(resp.Value) != 0 {
			return nil, false // an absence proof must not carry a value
		}
		wantValueHash = statetrie.ZeroHash()
	}
	if !bytes.Equal(resp.ValueHash, wantValueHash) {
		return nil, false
	}

	// (2) fold the branch back into the trusted root
	if !statetrie.Verify(resp.StateRoot, resp.Path, resp.ValueHash, resp.Proof) {
		return nil, false
	}

	if resp.Found {
		return resp.Value, true
	}
	return nil, true
}
