package rpcServer

import (
	"github.com/ngin-network/ngcore/chain"
	"net/http"
)

func NewVaultModule(vaultChain *chain.VaultChain) *Vault {
	return &Vault{
		vaultChain: vaultChain,
	}
}

type Vault struct {
	vaultChain *chain.VaultChain
}

func (v *Vault) DumpDB(r *http.Request, args *struct{}, reply *DumpKVReply) error {
	reply.KV = v.vaultChain.DB.GetAll()
	return nil
}
