package jsonrpc

import (
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (s *Server) submitBlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var block ngtypes.Block

	err := utils.JSON.Unmarshal(*msg.Params, &block)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.pow.MinedNewBlock(&block)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, block.Hash())
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug
func (s *Server) getBlockTemplateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	blockTemplate := s.pow.GetBlockTemplate()

	raw, err := utils.JSON.Marshal(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getWorkReply struct {
	RawHeader string `json:"raw"` // seed is in [17:49]
	RawBlock  string `json:"block"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug
func (s *Server) getWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	blockTemplate := s.pow.GetBlockTemplate()

	rawBlock, err := utils.Proto.Marshal(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var reply = getWorkReply{
		RawHeader: hex.EncodeToString(blockTemplate.GetPoWRawHeader(nil)),
		RawBlock:  hex.EncodeToString(rawBlock),
	}

	raw, err := utils.JSON.Marshal(reply)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type submitWorkParams struct {
	Nonce    string `json:"nonce"`
	RawBlock string `json:"block"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug
func (s *Server) submitWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params submitWorkParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	nonce, err := hex.DecodeString(params.Nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawBlock, err := hex.DecodeString(params.RawBlock)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var block ngtypes.Block
	err = utils.Proto.Unmarshal(rawBlock, &block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block.Nonce = nonce

	err = s.pow.MinedNewBlock(&block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}

type switchMiningParams struct {
	Mode string `json:"mode"`
}

// switchMiningFunc provides the switch on built-in block mining
func (s *Server) switchMiningFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params switchMiningParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	switch mode {
	case "on":
		log.Warn("switch mining on")
		s.pow.SwitchMiningOn()
	case "off":
		log.Warn("switch mining off")
		s.pow.SwitchMiningOff()
	default:
		workerNum, err := strconv.ParseInt(mode, 10, 64)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		log.Warnf("switch to %d mining workers", workerNum)
		s.pow.UpdateMiningThread(int(workerNum))
		s.pow.SwitchMiningOn()
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
