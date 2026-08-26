package jsonrpc

import (
	"encoding/hex"

	"github.com/c0mm4nd/go-jsonrpc2"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// getStateRootReply is the current consensus-state commitment root and the
// tip height it belongs to.
type getStateRootReply struct {
	Height    uint64 `json:"height"`
	StateRoot string `json:"stateRoot"` // lowercase hex of the 32-byte root
}

// getStateRootFunc returns the state commitment root at the current tip. It
// equals the tip block header's StateRoot (the two are pinned together by the
// apply-time CheckStateRoot), so a light client can trust either.
func (s *Server) getStateRootFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var root []byte
	err := s.pow.State.View(func(txn *bbolt.Tx) error {
		root = ngstate.StateRoot(txn)
		return nil
	})
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, getStateRootReply{
		Height:    s.pow.Chain.GetLatestBlock().GetHeight(),
		StateRoot: hex.EncodeToString(root),
	})
}

// getStateProofParams selects one leaf to prove. Key is bs58 for the
// address-keyed domains (balance/key/contract) or hex for the commit domain's
// raw heightLE(8)‖Hash(32) bucket key; a plain hex address is also accepted.
type getStateProofParams struct {
	Domain string `json:"domain"`
	Key    string `json:"key"`
}

// getStateProofReply is a self-contained Merkle proof: statetrie.Verify(
// stateRoot, path, valueHash, proof) accepts it. A zero valueHash proves the
// leaf is ABSENT. Value is the raw committed bytes (empty when absent).
type getStateProofReply struct {
	StateRoot string   `json:"stateRoot"`
	Path      string   `json:"path"`
	ValueHash string   `json:"valueHash"`
	Value     string   `json:"value"`
	Proof     []string `json:"proof"`
}

// getStateProofFunc builds a state-inclusion (or absence) proof for one leaf
// via statetrie.Prove. It round-trips against ng_getStateRoot: the returned
// stateRoot matches the tip root, and Verify folds the branch back into it.
func (s *Server) getStateProofFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getStateProofParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawKey, err := decodeStateKey(params.Domain, params.Key)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var (
		root, path, value, valueHash []byte
		proof                        [][]byte
	)
	err = s.pow.State.View(func(txn *bbolt.Tx) error {
		var e error
		root, path, value, valueHash, proof, e = ngstate.StateProof(txn, params.Domain, rawKey)
		return e
	})
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hexProof := make([]string, len(proof))
	for i, p := range proof {
		hexProof[i] = hex.EncodeToString(p)
	}

	return reply(msg, getStateProofReply{
		StateRoot: hex.EncodeToString(root),
		Path:      hex.EncodeToString(path),
		ValueHash: hex.EncodeToString(valueHash),
		Value:     hex.EncodeToString(value),
		Proof:     hexProof,
	})
}

// decodeStateKey resolves the domain's raw bucket key from the request. The
// address-keyed domains accept a bs58 address (or a 64-hex address); the
// commit domain takes the raw heightLE(8)‖Hash(32) key as hex.
func decodeStateKey(domain, key string) ([]byte, error) {
	if domain == "commit" {
		return hex.DecodeString(key)
	}
	// try bs58 first (the canonical address form), then fall back to hex
	if addr, err := ngtypes.NewAddressFromBS58(key); err == nil {
		return addr[:], nil
	}
	return hex.DecodeString(key)
}
