package miner

import (
	"math/big"
	"runtime"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/go-randomx"

	"github.com/NebulousLabs/fastrand"
	"go.uber.org/atomic"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("miner")

// Miner is an inner miner for proof of work
// miner implements a internal PoW miner with multi threads(goroutines) support
type Miner struct {
	ThreadNum int
	hashes    *atomic.Int64
	Job       *ngtypes.Block

	abortChs     []chan struct{}
	foundBlockCh chan *ngtypes.Block
}

// NewMiner will create a local miner which works in *ThreadNum* threads.
// when not mining, return a nil
func NewMiner(threadNum int, foundBlockCh chan *ngtypes.Block) *Miner {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if threadNum <= 0 {
		return nil
	}

	m := &Miner{
		ThreadNum:    threadNum,
		hashes:       atomic.NewInt64(0),
		Job:          nil,
		abortChs:     nil,
		foundBlockCh: foundBlockCh,
	}

	go func() {
		reportTicker := time.NewTicker(ngtypes.TargetTime)
		defer reportTicker.Stop()

		elapsed := int64(ngtypes.TargetTime / time.Second) // 16

		for {
			<-reportTicker.C

			go func() {
				hashes := m.hashes.Load()

				if m.Job != nil {
					log.Warnf("Total hashrate: %d h/s, height: %d, diff: %d",
						hashes/elapsed,
						m.Job.GetHeight(),
						new(big.Int).SetBytes(m.Job.GetDifficulty()))
				}

				m.hashes.Sub(hashes)
			}()
		}
	}()

	return m
}

// Mine will ignite the engine of Miner and all threads Mine working.
func (m *Miner) Mine(job *ngtypes.Block) {
	m.stop() // will close the former one, so no need to

	m.Job = job
	log.Info("mining on Job: block@%d diff: %s", job.Height, new(big.Int).SetBytes(job.Difficulty).String())

	m.abortChs = make([]chan struct{}, m.ThreadNum)
	for i := range m.abortChs {
		m.abortChs[i] = make(chan struct{})
	}

	cache, err := randomx.AllocCache(randomx.FlagDefault)
	if err != nil {
		panic(err)
	}

	randomx.InitCache(cache, job.PrevBlockHash)
	dataset, err := randomx.AllocDataset(randomx.FlagDefault)
	if err != nil {
		panic(err)
	}

	count := randomx.DatasetItemCount()
	var wg sync.WaitGroup
	var workerNum = uint32(runtime.NumCPU())
	for i := uint32(0); i < workerNum; i++ {
		wg.Add(1)
		a := (count * i) / workerNum
		b := (count * (i + 1)) / workerNum
		go func() {
			defer wg.Done()
			randomx.InitDataset(dataset, cache, a, b-a)
		}()
	}
	wg.Wait()

	var miningWG sync.WaitGroup
	for threadID := 0; threadID < m.ThreadNum; threadID++ {
		miningWG.Add(1)
		go func(threadID int) {
			defer miningWG.Done()
			diff := new(big.Int).SetBytes(job.GetDifficulty())
			target := new(big.Int).Div(ngtypes.MaxTarget, diff)

			vm, err := randomx.CreateVM(cache, dataset, randomx.FlagDefault)
			if err != nil {
				panic(err)
			}
			defer randomx.DestroyVM(vm)
			for {
				select {
				case <-m.abortChs[threadID]:
					// Mining terminated, update stats and abort
					return
				default:
					if !job.IsUnsealing() {
						continue
					}

					// Compute the PoW value of this nonce
					nonce := make([]byte, ngtypes.NonceSize)
					fastrand.Read(nonce)

					hash := randomx.CalculateHash(vm, job.GetPoWRawHeader(nonce))

					m.hashes.Inc()

					if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
						go m.found(threadID, job, nonce)

						return
					}
				}
			}
		}(threadID)
	}
	miningWG.Wait()

	randomx.ReleaseCache(cache)
	randomx.ReleaseDataset(dataset)
}

// Stop will Stop all workers via closing all channels.
func (m *Miner) Stop() {
	m.Job = nil // nil value

	var wg sync.WaitGroup
	for i := range m.abortChs {
		wg.Add(1)
		go func(i int) {
			close(m.abortChs[i])
			<-m.abortChs[i]
			wg.Done()
		}(i)
	}

	log.Info("mining mode off")
}

func (m *Miner) stop() {
	m.Job = nil // nil value

	var wg sync.WaitGroup
	for i := range m.abortChs {
		wg.Add(1)
		go func(i int) {
			m.abortChs[i] <- struct{}{}
			wg.Done()
		}(i)
	}
}

func (m *Miner) found(t int, job *ngtypes.Block, nonce []byte) {
	// Correct nonce found
	log.Debugf("Thread %d found nonce %x", t, nonce)

	block, err := job.ToSealed(nonce)
	if err != nil {
		panic(err)
	}

	m.foundBlockCh <- block
}
