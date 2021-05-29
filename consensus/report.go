package consensus

import (
	"sync"
	"time"
)

var reporterOnce sync.Once

func (pow *PoWork) reportLoop() {
	reporterOnce.Do(func() {
		interval := time.NewTicker(time.Minute)
		for {
			<-interval.C
			latestBlock := pow.Chain.GetLatestBlock()
			log.Warnf("local latest block@%d: %x", latestBlock.Height, latestBlock.GetHash())
		}
	})
}
