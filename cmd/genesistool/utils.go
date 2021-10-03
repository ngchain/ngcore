package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"runtime"
	"sync"

	"github.com/ngchain/go-randomx"

	"github.com/ngchain/ngcore/ngtypes"
)

func genBlockNonce(b *ngtypes.Block) {
	diff := new(big.Int).SetBytes(b.Header.Difficulty)
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

func calcHash(b *ngtypes.Block, target *big.Int, answerCh chan []byte, stopCh chan struct{}) {
	// calcHash get the hash of block
	cache, err := randomx.AllocCache(randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	randomx.InitCache(cache, b.Header.PrevBlockHash)
	ds, err := randomx.AllocDataset(randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	count := randomx.DatasetItemCount()
	var wg sync.WaitGroup
	workerNum := uint32(runtime.NumCPU())
	for i := uint32(0); i < workerNum; i++ {
		wg.Add(1)
		a := (count * i) / workerNum
		b := (count * (i + 1)) / workerNum
		go func() {
			defer wg.Done()
			randomx.InitDataset(ds, cache, a, b-a)
		}()
	}
	wg.Wait()

	vm, err := randomx.CreateVM(cache, ds, randomx.FlagJIT)
	if err != nil {
		panic(err)
	}

	random := make([]byte, ngtypes.NonceSize)

	for {
		select {
		case <-stopCh:
			return
		default:
			rand.Read(random)
			blob := b.GetPoWRawHeader(random)

			hash := randomx.CalculateHash(vm, blob)
			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				answerCh <- random
				return
			}
		}
	}
}
