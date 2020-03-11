package rpcServer

import (
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/ngtypes"
	"net/http"
)

func NewBlockModule(blockChain *chain.BlockChain) *Block {
	return &Block{
		blockChain: blockChain,
	}
}

type Block struct {
	blockChain *chain.BlockChain
}

type DumpReply struct {
	Table map[string]*ngtypes.Block `json:"table"`
}

func (b *Block) DumpMem(r *http.Request, args *struct{}, reply *DumpReply) error {
	table := make(map[string]*ngtypes.Block)
	b.blockChain.Mem.HashMap.Range(func(hash, block interface{}) bool {
		table[hash.(string)] = block.(*ngtypes.Block)
		return true
	})

	reply.Table = table

	return nil
}

type DumpKVReply struct {
	KV map[string]interface{}
}

func (b *Block) DumpDB(r *http.Request, args *struct{}, reply *DumpKVReply) error {
	reply.KV = b.blockChain.DB.GetAll()
	return nil
}
