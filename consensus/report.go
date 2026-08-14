package consensus

import (
	"runtime"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

func (pow *PoWork) reportLoop() {
	interval := time.NewTicker(time.Minute)
	defer interval.Stop()

	for {
		select {
		case <-interval.C:
			latestBlock := pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
			log.Warnf("local latest block@%d: %x", latestBlock.GetHeight(), latestBlock.GetHash())
			runtime.Gosched()
		case <-pow.ctx.Done():
			return
		}
	}
}
