package rpc

import (
	"context"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/ngchain/ngcore/ngp2p"
	"net/http"
)

type P2P struct {
	p2p *ngp2p.LocalNode
}

func NewP2PModule(p2p *ngp2p.LocalNode) *P2P {
	return &P2P{
		p2p: p2p,
	}
}

type AddNodeArgs struct {
	Addr string `json:"addr"`
}

type BoolResultReply struct {
	Result bool `json:"result"`
}

func (p2p *P2P) AddNode(r *http.Request, args *AddNodeArgs, reply *BoolResultReply) error {
	targetAddr, err := multiaddr.NewMultiaddr(args.Addr)
	if err != nil {
		return err
	}

	targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		return err
	}

	err = p2p.p2p.Connect(context.Background(), *targetInfo)
	if err != nil {
		return err
	}

	p2p.p2p.Ping(targetInfo.ID)
	return nil
}
