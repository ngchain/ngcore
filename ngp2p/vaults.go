package ngp2p

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngtypes"
)

func (p *Protocol) Vaults(s network.Stream, uuid string, getblocks *ngtypes.GetBlocksPayload) bool {

	return true
}

func (p *Protocol) onVaults(s network.Stream) {

}
