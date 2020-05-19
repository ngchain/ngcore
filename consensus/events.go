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
			err := storage.GetChain().PutNewBlock(block)
			if err != nil {
				log.Warnf("failed to put new block from p2p network: %s", err)
			}
		case tx := <-ngp2p.GetLocalNode().OnTx:
			err := txpool.GetTxPool().PutTxs(tx)
			if err != nil {
				log.Warnf("failed to put new tx from p2p network: %s", err)
			}
		}
	}
}
