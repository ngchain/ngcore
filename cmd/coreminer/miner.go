package main

import (
	"crypto/rand"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ngchain/go-randomx"
	"github.com/ngchain/ngcore/ngtypes"
	"go.uber.org/atomic"
)

type Task struct {
	threadNum int

	hashes     *atomic.Int64
	quitChPool []chan struct{}
	foundCh    chan Job
	AllExitCh  chan struct{}
}

func NewMiner(threadNum int, foundCh chan Job, allExitCh chan struct{}) *Task {
	if threadNum <= 0 {
		panic("thread number is incorrect")
	}

	log.Printf("start mining with %d thread(s)", threadNum)

	m := &Task{
		threadNum:  threadNum,
		hashes:     atomic.NewInt64(0),
		foundCh:    foundCh,
		quitChPool: make([]chan struct{}, threadNum),
		AllExitCh:  allExitCh,
	}

	go func() {
		interval := 10 * time.Second
		reportTicker := time.NewTicker(interval)
		defer reportTicker.Stop()

		elapsed := int64(interval / time.Second) // 60

		for {
			<-reportTicker.C

			hashes := m.hashes.Load()
			log.Printf("Total hashrate: %d h/s", hashes/elapsed)

			m.hashes.Sub(hashes)
		}
	}()

	return m
}

func (m *Task) Mining(work Job) {
	diff := new(big.Int).SetBytes(work.Header.Difficulty)
	target := new(big.Int).Div(ngtypes.MaxTarget, diff)

	cache, err := randomx.AllocCache()
	if err != nil {
		panic(err)
	}
	defer randomx.ReleaseCache(cache)

	randomx.InitCache(cache, work.Header.PrevBlockHash)

	dataset, err := randomx.AllocDataset()
	if err != nil {
		panic(err)
	}
	defer randomx.ReleaseDataset(dataset)

	count := randomx.DatasetItemCount()
	randomx.InitDataset(dataset, cache, 0, count)
	// var wg sync.WaitGroup
	// var workerNum = uint32(runtime.NumCPU())
	// for i := uint32(0); i < workerNum; i++ {
	// 	wg.Add(1)
	// 	a := (count * i) / workerNum
	// 	b := (count * (i + 1)) / workerNum
	// 	go func() {
	// 		defer wg.Done()
	// 		randomx.InitDataset(dataset, cache, a, b-a)
	// 	}()
	// }
	// wg.Wait()
	log.Println("mining ready")

	var miningWG sync.WaitGroup
	for threadID := 0; threadID < m.threadNum; threadID++ {
		miningWG.Add(1)
		m.quitChPool[threadID] = make(chan struct{})

		go func(threadID int) {
			defer miningWG.Done()

			vm, err := randomx.CreateVM(cache, dataset, randomx.FlagJIT, randomx.FlagHardAES, randomx.FlagFullMEM)
			if err != nil {
				panic(err)
			}

			defer randomx.DestroyVM(vm)

			for {
				select {
				case <-m.quitChPool[threadID]:
					return
				default:
					// Compute the PoW value of this nonce
					nonce := make([]byte, 8)
					_, err := rand.Read(nonce)
					if err != nil {
						return
					}

					hash := randomx.CalculateHash(vm, work.GetPoWRawHeader(nonce))

					m.hashes.Inc()

					if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
						log.Printf("thread %d found nonce %x for block @ %d", threadID, nonce, work.Header.Height)
						work.SetNonce(nonce)
						m.foundCh <- work
						return
					}
				}
			}
		}(threadID)
	}
	miningWG.Wait()
	m.AllExitCh <- struct{}{}
}
