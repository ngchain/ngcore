package consensus

import (
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

func (pow *PoWork) loop() {
	for {
		if ngp2p.GetLocalNode().OnBlock == nil || ngp2p.GetLocalNode().OnTx == nil {
			panic("event chan is nil")
		}

		select {
		case block := <-ngp2p.GetLocalNode().OnBlock:
			storage.GetChain().PutNewBlock(block)
		case tx := <-ngp2p.GetLocalNode().OnTx:
			txpool.GetTxPool().PutTxs(tx)
		}
	}
}
