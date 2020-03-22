package rpc

import (
	"github.com/ngchain/ngcore/ngchain"
	"net/http"
)

func NewChainModule(chain *ngchain.Chain) *Chain {
	return &Chain{
		chain: chain,
	}
}

type Chain struct {
	chain *ngchain.Chain
}

/* Chain */
type DumpAllByHeightReply struct {
	Table map[string]ngchain.Item `json:"table"`
}

func (c *Chain) DumpAllByHash(r *http.Request, args *struct{}, reply *DumpAllByHeightReply) error {
	reply.Table = c.chain.DumpAllByHash(true, true)
	return nil
}
