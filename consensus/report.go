package consensus

import (
	"github.com/ngchain/ngcore/ngtypes"
	"runtime"
	"time"
)

func (pow *PoWork) reportLoop() {
	interval := time.NewTicker(time.Minute)
	for {
		<-interval.C
		latestBlock := pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
		log.Warnf("local latest block@%d: %x", latestBlock.GetHeight(), latestBlock.GetHash())
		runtime.Gosched()
	}
}
