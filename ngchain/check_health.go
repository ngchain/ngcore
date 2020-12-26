package ngchain

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

func (chain *Chain) CheckHealth(network ngtypes.NetworkType) {
	log.Warn("checking chain's health")
	latestHeight := chain.GetLatestBlockHeight()

	origin := chain.GetOriginBlock()
	originHeight := origin.Height

	prevBlockHash := origin.Hash()

	for h := originHeight; h < latestHeight; {
		h++
		b, err := chain.GetBlockByHeight(h)
		if err != nil {
			panic(err)
		}

		if !bytes.Equal(b.PrevBlockHash, prevBlockHash) {
			panic(fmt.Errorf("prev block hash %x is incorrect, shall be %x", b.PrevBlockHash, prevBlockHash))
		}

		prevBlockHash = b.Hash()
	}

	log.Warn("checking is finished")
}
