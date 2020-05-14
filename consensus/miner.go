package consensus

import (
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/NebulousLabs/fastrand"
	"github.com/ngchain/cryptonight-go"
	"go.uber.org/atomic"

	"github.com/ngchain/ngcore/ngtypes"
)

// minerModule is an inner miner for proof of work
// miner implements a internal PoW miner with multi threads(goroutines) support
type minerModule struct {
	pow *PoWork

	threadNum int
	hashes    atomic.Int64

	isRunning    *atomic.Bool // bool
	abortCh      chan struct{}
	FoundBlockCh chan *ngtypes.Block
}

// newMinerModule will create a local miner which works in *threadNum* threads.
func newMinerModule(pow *PoWork, threadNum int) *minerModule {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if threadNum < 0 {
		return nil
	}

	if threadNum == 0 {
		threadNum = runtime.NumCPU()
	}

	m := &minerModule{
		pow:          pow,
		isRunning:    atomic.NewBool(false),
		threadNum:    threadNum,
		abortCh:      make(chan struct{}),
		FoundBlockCh: make(chan *ngtypes.Block),
	}

	go func() {
		reportTicker := time.NewTicker(time.Minute)
		defer reportTicker.Stop()

		elapsed := int64(time.Minute / time.Second) // 60

		for {
			<-reportTicker.C

			go func() {
				hashes := m.hashes.Load()
				log.Infof("Total Hashrate: %d h/s", hashes/elapsed)
				m.hashes.Add(-hashes)
			}()
		}
	}()

	return m
}

// Start will ignite the engine of Miner and all threads start working.
func (m *minerModule) start(initJob *ngtypes.Block) {
	m.isRunning.Store(true)
	m.abortCh = make(chan struct{})

	once := new(sync.Once)

	for threadID := 0; threadID < m.threadNum; threadID++ {
		go m.mine(threadID, initJob, once)
	}
}

// stop will stop all threads. It would lose some hashrate, but it's necessary in a node for stablity.
func (m *minerModule) stop() {
	if !m.isRunning.Load() {
		return
	}

	m.isRunning.Store(false)
	close(m.abortCh)
	<-m.abortCh // wait
}

func (m *minerModule) mine(threadID int, job *ngtypes.Block, once *sync.Once) {
	diff := new(big.Int).SetBytes(job.GetHeader().GetDifficulty())
	target := new(big.Int).Div(ngtypes.MaxTarget, diff)

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
			nonce := make([]byte, 4)
			fastrand.Read(nonce)

			hash := cryptonight.Sum(job.GetPoWBlob(nonce), 0)

			m.hashes.Inc()

			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				go once.Do(func() {
					m.found(threadID, job, nonce)
				})

				return
			}
		}
	}
}

func (m *minerModule) found(t int, job *ngtypes.Block, nonce []byte) {
	// Correct nonce found
	m.stop()

	log.Debugf("Thread %d found nonce %x", t, nonce)

	block, err := job.ToSealed(nonce)
	if err != nil {
		log.Panic(err)
	}

	pow.minedNewBlock(block)
	// assign new job
	pow.minerMod.start(pow.getBlockTemplate())
}
