package main

import (
	"crypto/rand"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/deroproject/astrobwt"
	"github.com/ngchain/ngcore/ngtypes"
	"go.uber.org/atomic"
)

type Task struct {
	running   *atomic.Bool
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

	quitChPool := make([]chan struct{}, threadNum)
	for i := range quitChPool {
		quitChPool[i] = make(chan struct{}, 1)
	}

	m := &Task{
		running:    atomic.NewBool(false),
		threadNum:  threadNum,
		hashes:     atomic.NewInt64(0),
		foundCh:    foundCh,
		quitChPool: quitChPool,
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

func (t *Task) Mining(work Job) {
	ok := t.running.CAS(false, true)
	if !ok {
		panic("try over mining")
	}

	diff := new(big.Int).SetBytes(work.BlockHeader.Difficulty)
	target := new(big.Int).Div(ngtypes.MaxTarget, diff)

	log.Println("mining ready")

	var miningWG sync.WaitGroup
	for threadID := 0; threadID < t.threadNum; threadID++ {
		miningWG.Add(1)

		go func(threadID int) {
			defer miningWG.Done()

			for {
				select {
				case <-t.quitChPool[threadID]:
					return
				default:
					// Compute the PoW value of this nonce
					nonce := make([]byte, 8)
					_, err := rand.Read(nonce)
					if err != nil {
						return
					}

					hash := astrobwt.POW_0alloc(work.GetPoWRawHeader(nonce))

					t.hashes.Inc()

					if new(big.Int).SetBytes(hash[:]).Cmp(target) < 0 {
						log.Printf("thread %d found nonce %x for block @ %d", threadID, nonce, work.GetHeight())
						work.SetNonce(nonce)
						t.foundCh <- work
						return
					}
				}
			}
		}(threadID)
	}
	miningWG.Wait()
	t.AllExitCh <- struct{}{}
}

func (t *Task) ExitJob() {
	ok := t.running.CAS(true, false)
	if ok {
		for i := range t.quitChPool {
			t.quitChPool[i] <- struct{}{}
		}

		<-t.AllExitCh

		for i := range t.quitChPool {
			t.quitChPool[i] = make(chan struct{}, 1)
		}
	}
}
