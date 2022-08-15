package main

import (
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/secp256k1"
)

type Job struct {
	block  *ngtypes.FullBlock
	WorkID uint64
	Nonce  []byte
	GenTx  string
}

func NewJob(network ngtypes.Network, priv *secp256k1.PrivateKey, reply *jsonrpc.GetWorkReply) *Job {
	var block ngtypes.FullBlock
	var txs []*ngtypes.FullTx
	err := utils.HexRLPDecode(reply.Block, &block)
	if err != nil {
		panic(err)
	}
	err = utils.HexRLPDecode(reply.Txs, &txs)
	if err != nil {
		panic(err)
	}

	extraData := []byte("coreminer")
	genTx := consensus.CreateGenerateTx(network, priv, block.Height, extraData)

	block.Txs = append([]*ngtypes.FullTx{genTx}, txs...)

	return &Job{
		block:  &block,
		WorkID: reply.WorkID,
		Nonce:  nil,
		GenTx:  utils.HexRLPEncode(genTx),
	}
}

func (j *Job) SetNonce(nonce []byte) {
	j.Nonce = nonce
}
