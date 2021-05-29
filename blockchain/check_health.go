package blockchain

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
)

func (chain *Chain) CheckHealth(network ngproto.NetworkType) {
	log.Warn("checking chain's health")
	latestHeight := chain.GetLatestBlockHeight()

	origin := chain.GetOriginBlock()
	originHeight := origin.Header.GetHeight()

	prevBlockHash := origin.GetHash()

	for h := originHeight; h < latestHeight; {
		h++
		b, err := chain.GetBlockByHeight(h)
		if err != nil {
			panic(err)
		}

		if !bytes.Equal(b.Header.GetPrevBlockHash(), prevBlockHash) {
			panic(fmt.Errorf("prev block hash %x is incorrect, shall be %x", b.Header.GetPrevBlockHash(), prevBlockHash))
		}

		prevBlockHash = b.GetHash()
	}

	log.Warn("checking is finished")
}
