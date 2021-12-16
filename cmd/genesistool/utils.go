package main

import (
	"crypto/rand"
	"fmt"
	"github.com/ngchain/astrobwt"
	"math/big"
	"runtime"

	"github.com/ngchain/ngcore/ngtypes"
)

func genBlockNonce(b *ngtypes.FullBlock) {
	diff := new(big.Int).SetBytes(b.BlockHeader.Difficulty)
	genesisTarget := new(big.Int).Div(ngtypes.MaxTarget, diff)
	fmt.Printf("Genesis block's diff %d, target %x \n", diff, genesisTarget.Bytes())

	nCh := make(chan []byte, 1)
	stopCh := make(chan struct{}, 1)
	thread := runtime.NumCPU()

	for i := 0; i < thread; i++ {
		go calcHash(b, genesisTarget, nCh, stopCh)
	}

	answer := <-nCh
	stopCh <- struct{}{}

	fmt.Printf("Genesis Block Nonce Hex: %x \n", answer)
}

func calcHash(b *ngtypes.FullBlock, target *big.Int, answerCh chan []byte, stopCh chan struct{}) {
	// calcHash get the hash of block

	random := make([]byte, ngtypes.NonceSize)

	for {
		select {
		case <-stopCh:
			return
		default:
			rand.Read(random)
			blob := b.GetPoWRawHeader(random)

			hash := astrobwt.POW_0alloc(blob[:])
			if new(big.Int).SetBytes(hash[:]).Cmp(target) < 0 {
				answerCh <- random
				return
			}
		}
	}
}
