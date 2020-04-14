package miner

import (
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/NebulousLabs/fastrand"
	logging "github.com/ipfs/go-log"
	"github.com/ngchain/cryptonight-go"
	"go.uber.org/atomic"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("miner")

// Miner is an inner miner for proof of work
type Miner struct {
	threadNum int
	hashes    atomic.Int64

	isRunning    *atomic.Bool // bool
	abortCh      chan struct{}
	FoundBlockCh chan *ngtypes.Block
}

// NewMiner will create a local miner which works in *threadNum* threads
func NewMiner(threadNum int) *Miner {
	runtime.GOMAXPROCS(runtime.NumCPU())

	m := &Miner{
		isRunning:    atomic.NewBool(false),
		threadNum:    threadNum,
		abortCh:      make(chan struct{}),
		FoundBlockCh: make(chan *ngtypes.Block),
	}

	go func() {
		reportTicker := time.NewTicker(time.Minute)
		defer reportTicker.Stop()

		elapsed := int64(60)

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

// Start will ignite the engine of Miner and all threads start working
func (m *Miner) Start(initJob *ngtypes.Block) {
	if m.isRunning.Load() {
		return
	}
	m.isRunning.Store(true)
	m.abortCh = make(chan struct{})

	once := new(sync.Once)
	for threadID := 0; threadID < m.threadNum; threadID++ {
		go m.mine(threadID, initJob, once)
	}
}

// Stop will stop all threads. It would lose some hashrate, but it's necessary in a node for stablity
func (m *Miner) Stop() {
	if !m.isRunning.Load() {
		return
	}

	m.isRunning.Store(false)
	close(m.abortCh)
	<-m.abortCh // wait
}

func (m *Miner) mine(threadID int, job *ngtypes.Block, once *sync.Once) {
	var target = new(big.Int).SetBytes(job.Header.Target)

	for {
		select {
		case <-m.abortCh:
			// Mining terminated, update stats and abort
			return
		default:
			if job == nil || !job.IsUnsealing() {
				continue
			}

			// Compute the PoW value of this nonce
			nonce := make([]byte, 4)
			fastrand.Read(nonce)
			blob := job.GetPoWBlob(nonce)
			hash := cryptonight.Sum(blob, 0)
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

func (m *Miner) found(t int, job *ngtypes.Block, nonce []byte) {
	// Correct nonce found
	m.Stop()

	log.Debugf("Thread %d found nonce %x", t, nonce)
	block, err := job.ToSealed(nonce)
	if err != nil {
		log.Panic(err)
	}

	m.FoundBlockCh <- block
}
