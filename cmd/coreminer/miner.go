package main

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	"github.com/ngchain/astrobwt"

	"github.com/ngchain/ngcore/ngtypes"
	"go.uber.org/atomic"
)

type Miner struct {
	running   *atomic.Bool
	threadNum int

	hashes     *atomic.Int64
	quitSignal *atomic.Bool
	foundCh    chan Job
	AllExitCh  chan struct{}
}

func NewMiner(threadNum int, foundCh chan Job, allExitCh chan struct{}) *Miner {
	if threadNum <= 0 {
		panic("thread number is incorrect")
	}

	log.Warningf("start mining with %d thread(s)", threadNum)

	quitSignal := atomic.NewBool(false)

	m := &Miner{
		running:    atomic.NewBool(false),
		threadNum:  threadNum,
		hashes:     atomic.NewInt64(0),
		foundCh:    foundCh,
		quitSignal: quitSignal,
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
			log.Warningf("Total hashrate: %d h/s", hashes/elapsed)

			m.hashes.Sub(hashes)
		}
	}()

	return m
}

func (t *Miner) Mining(work Job) {
	ok := t.running.CompareAndSwap(false, true)
	if !ok {
		panic("try over mining")
	}

	diff := new(big.Int).SetBytes(work.block.BlockHeader.Difficulty)
	target := new(big.Int).Div(ngtypes.MaxTarget, diff)

	log.Warn("mining ready")

	var miningWG sync.WaitGroup
	for threadID := 0; threadID < t.threadNum; threadID++ {
		miningWG.Add(1)

		go func(threadID int) {
			defer miningWG.Done()

			for {
				if t.quitSignal.Load() {
					return
				}

				// Compute the PoW value of this nonce
				nonce := make([]byte, 8)
				_, err := rand.Read(nonce)
				if err != nil {
					return
				}

				hash := astrobwt.POW_0alloc(work.block.GetPoWRawHeader(nonce))

				t.hashes.Inc()

				if hash != [32]byte{} && new(big.Int).SetBytes(hash[:]).Cmp(target) < 0 {
					log.Warningf("thread %d found nonce %x for block @ %d", threadID, nonce, work.block.GetHeight())
					log.Debugf("%s < %s", new(big.Int).SetBytes(hash[:]), target)
					work.SetNonce(nonce)
					t.foundCh <- work
					return
				}
				// }
			}
		}(threadID)
	}
	miningWG.Wait()
	t.AllExitCh <- struct{}{}
}

func (t *Miner) ExitJob() {
	ok := t.running.CompareAndSwap(true, false)
	if ok {
		log.Warn("exiting all jobs")
		t.quitSignal.Store(true)
		<-t.AllExitCh
		t.quitSignal.Store(false)
	}
}
