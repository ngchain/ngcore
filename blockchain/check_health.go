package blockchain

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

func (chain *Chain) CheckHealth(network ngtypes.Network) {
	log.Warn("checking chain's health")
	latestHeight := chain.GetLatestBlockHeight()

	origin := chain.GetOriginBlock()
	originHeight := origin.GetHeight()

	prevBlockHash := origin.GetHash()

	for h := originHeight; h < latestHeight; {
		h++
		b, err := chain.getBlockByHeight(h)
		if err != nil {
			panic(err)
		}

		if !bytes.Equal(b.GetPrevHash(), prevBlockHash) {
			panic(fmt.Sprintf("prev block hash %x is incorrect, shall be %x", b.GetPrevHash(), prevBlockHash))
		}

		prevBlockHash = b.GetHash()
	}

	log.Warn("checking is finished")
}
