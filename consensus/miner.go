package consensus

import (
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/ngchain/go-randomx"

	"github.com/NebulousLabs/fastrand"
	"go.uber.org/atomic"

	"github.com/ngchain/ngcore/ngtypes"
)

// minerModule is an inner miner for proof of work
// miner implements a internal PoW miner with multi threads(goroutines) support
type minerModule struct {
	pow *PoWork

	threadNum int
	hashes    atomic.Int64
	job       *atomic.Value

	abortCh      chan struct{}
	FoundBlockCh chan *ngtypes.Block
}

// newMinerModule will create a local miner which works in *threadNum* threads.
// when not mining, return a nil
func newMinerModule(pow *PoWork, threadNum int) *minerModule {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if threadNum <= 0 {
		return nil
	}

	m := &minerModule{
		pow:          pow,
		job:          new(atomic.Value),
		threadNum:    threadNum,
		abortCh:      make(chan struct{}),
		FoundBlockCh: make(chan *ngtypes.Block),
	}

	go func() {
		reportTicker := time.NewTicker(ngtypes.TargetTime)
		defer reportTicker.Stop()

		elapsed := int64(ngtypes.TargetTime / time.Second) // 60

		for {
			<-reportTicker.C

			go func() {
				hashes := m.hashes.Load()

				if m.job.Load() != nil {
					current, _ := m.job.Load().(*ngtypes.Block)
					log.Warnf("Total Hashrate: %d h/s, Current Job: block@%d, diff: %d", hashes/elapsed, current.GetHeight(), new(big.Int).SetBytes(current.GetDifficulty()))
				}

				m.hashes.Sub(hashes)
			}()
		}
	}()

	return m
}

// Start will ignite the engine of Miner and all threads start working.
// and can also be used as update job
func (m *minerModule) start(job *ngtypes.Block) {
	if m.job.Load() != nil {
		log.Info("mining job updeting")
	} else {
		log.Info("mining mod on")
	}

	m.job.Store(job)

	m.abortCh = make(chan struct{})
	once := new(sync.Once)

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
	for threadID := 0; threadID < m.threadNum; threadID++ {
		miningWG.Add(1)
		go func(threadID int) {
			defer miningWG.Done()
			diff := new(big.Int).SetBytes(job.GetDifficulty())
			target := new(big.Int).Div(ngtypes.MaxTarget, diff)

			vm, err := randomx.CreateVM(cache, dataset, randomx.FlagDefault)
			if err != nil {
				panic(err)
			}

			for {
				select {
				case <-m.abortCh:
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
						go once.Do(func() {
							m.found(threadID, job, nonce)
						})

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

// stop will stop all threads. It would lose some hashrate, but it's necessary in a node for stablity.
func (m *minerModule) stop() {
	if m.job.Load() == nil {
		return
	}

	m.job = new(atomic.Value) // nil value
	close(m.abortCh)
	<-m.abortCh // wait

	log.Info("mining mode off")
}

func (m *minerModule) found(t int, job *ngtypes.Block, nonce []byte) {
	// Correct nonce found
	m.stop()

	log.Debugf("Thread %d found nonce %x", t, nonce)

	block, err := job.ToSealed(nonce)
	if err != nil {
		log.Panic(err)
	}

	err = pow.MinedNewBlock(block)
	if err != nil {
		log.Warn(err) // may have "has block in same height" error here
	}
	// assign new job
	m.start(pow.GetBlockTemplate())
}
