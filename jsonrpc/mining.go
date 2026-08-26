package jsonrpc

import (
	"encoding/hex"
	"strconv"
	"time"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/jsonrpc/workpool"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// type GetWorkParams struct {
// 	PrivateKey string `json:"private_key"`
// }

var workPool = workpool.GetWorkerPool()

// GetWorkParams carries the miner's OWN signed generate tx. The post-state
// StateRoot is folded into the pow preimage and depends on the block's
// contents including this generate, so getWork must fold it in and seal the
// root BEFORE returning — the miner can no longer append its generate after
// the fact and still carry a correct root.
type GetWorkParams struct {
	GenTx string `json:"gen"`
}

// GetWorkReply hands the miner a FULLY-ASSEMBLED unsealing block (generate
// folded in, StateRoot sealed) — it needs only a nonce. Txs is kept as an
// empty list for wire-compat but is no longer needed to reconstruct.
type GetWorkReply struct {
	WorkID uint64 `json:"id"`
	Block  string `json:"block"`
	Txs    string `json:"txs"`
}

// getWorkFunc provides a free style interface for miner client getting latest
// block mining work. It requires the miner's signed generate so the returned
// template already carries the correct post-state StateRoot in its preimage.
func (s *Server) getWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params GetWorkParams
	if msg.Params != nil {
		if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
	}

	var genTx ngtypes.FullTx
	if err := utils.HexRLPDecode(params.GenTx, &genTx); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := s.pow.AssembleWork(&genTx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	id := uint64(time.Now().UnixNano())
	work := &GetWorkReply{
		WorkID: id,
		Block:  utils.HexRLPEncode(block),
		Txs:    utils.HexRLPEncode([]*ngtypes.FullTx{}),
	}

	// stash the assembled block itself: submitWork only needs to seal a nonce
	workPool.Put(strconv.FormatUint(work.WorkID, 10), block)

	return reply(msg, work)
}

// SubmitWorkParams carries only the found nonce for a stored work id: the
// block (with its generate and StateRoot) was fully assembled at getWork time.
type SubmitWorkParams struct {
	WorkID uint64 `json:"id"`
	Nonce  string `json:"nonce"`
}

// submitWorkFunc seals the stored, fully-assembled block with the found nonce
// and imports it. It no longer reconstructs the block from parts — the root
// was sealed at getWork time, so a re-assembly here could only diverge.
func (s *Server) submitWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params SubmitWorkParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	stored, err := workPool.Get(strconv.FormatUint(params.WorkID, 10))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	nonce, err := hex.DecodeString(params.Nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	// clone the stored block so a re-submit under the same work id starts from
	// the unsealed template rather than a previously-sealed one
	tmpl := stored.(*ngtypes.FullBlock)
	block := *tmpl
	hdr := *tmpl.BlockHeader
	block.BlockHeader = &hdr

	err = block.ToSealed(nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.pow.MinedNewBlock(&block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
