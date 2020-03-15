package rpcServer

import (
	"github.com/ngin-network/ngcore/chain"
	"net/http"
)

func NewChainModule(chain *chain.Chain) *Chain {
	return &Chain{
		chain: chain,
	}
}

type Chain struct {
	chain *chain.Chain
}

/* Chain */
type DumpAllByHeightReply struct {
	Table map[string]chain.Item `json:"table"`
}

func (c *Chain) DumpAllByHash(r *http.Request, args *struct{}, reply *DumpAllByHeightReply) error {
	reply.Table = c.chain.DumpAllByHash(true, true)
	return nil
}
