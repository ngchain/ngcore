package consensus

import (
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
)

func (pow *PoWork) loop() {
	for {
		if ngp2p.GetLocalNode().OnBlock == nil || ngp2p.GetLocalNode().OnTx == nil {
			panic("event chan is nil")
		}

		select {
		case block := <-ngp2p.GetLocalNode().OnBlock:
			err := pow.ApplyBlock(block)
			if err != nil {
				log.Warnf("failed to put new block from p2p network: %s", err)
			}
		case tx := <-ngp2p.GetLocalNode().OnTx:
			err := ngstate.GetTxPool().PutTx(tx)
			if err != nil {
				log.Warnf("failed to put new tx from p2p network: %s", err)
			}
		}
	}
}
