package consensus

import (
	"runtime"
	"time"
)

func (pow *PoWork) reportLoop() {
	interval := time.NewTicker(time.Minute)
	for {
		<-interval.C
		latestBlock := pow.Chain.GetLatestBlock()
		log.Warnf("local latest block@%d: %x", latestBlock.Header.Height, latestBlock.GetHash())
		runtime.Gosched()
	}
}
