package main

import (
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type Job struct {
	block  *ngtypes.FullBlock
	WorkID uint64
	Nonce  []byte
}

// NewJob decodes a fully-assembled work template. The daemon now folds the
// miner's generate in and seals the post-state StateRoot into the header
// BEFORE returning (both are part of the pow preimage), so the miner only has
// to grind a nonce over the block as-is — no re-assembly here.
func NewJob(network ngtypes.Network, reply *jsonrpc.GetWorkReply) *Job {
	var block ngtypes.FullBlock
	err := utils.HexRLPDecode(reply.Block, &block)
	if err != nil {
		panic(err)
	}

	log.Warnf("new work: block %d, txTrie %x, stateRoot %x",
		block.Height, block.TxTrieHash, block.StateRoot)

	return &Job{
		block:  &block,
		WorkID: reply.WorkID,
		Nonce:  nil,
	}
}

func (j *Job) SetNonce(nonce []byte) {
	j.Nonce = nonce
}
