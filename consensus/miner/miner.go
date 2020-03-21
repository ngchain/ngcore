package miner

import (
	"github.com/NebulousLabs/fastrand"
	"github.com/ngin-network/cryptonight-go"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var log = logging.MustGetLogger("miner")

// Miner
type Miner struct {
	threadNum int
	hashes    int64

	status  *atomic.Value
	abortCh chan struct{}
	foundCh chan *ngtypes.Block
	mu      sync.Mutex
	wg      *sync.WaitGroup
}

func NewMiner(threadNum int, foundCh chan *ngtypes.Block) *Miner {
	runtime.GOMAXPROCS(runtime.NumCPU())

	status := new(atomic.Value)
	status.Store(false)
	m := &Miner{
		status:    status,
		threadNum: threadNum,
		abortCh:   make(chan struct{}),
		foundCh:   foundCh,
		wg:        new(sync.WaitGroup),
	}

	reportCh := time.Tick(time.Minute)
	elapsed := int64(60)

	go func() {
		for {
			select {
			case <-reportCh:
				go func() {
					hashes := atomic.LoadInt64(&m.hashes)
					log.Infof("Total Hashrate: %d h/s", hashes/elapsed)
					atomic.AddInt64(&m.hashes, -hashes)
				}()
			}
		}
	}()

	return m
}

func (m *Miner) Start(initJob *ngtypes.Block) {
	if m.status.Load().(bool) == true {
		return
	}
	m.status.Store(true)
	for threadID := 0; threadID < m.threadNum; threadID++ {
		go m.mine(threadID, initJob)
	}
}

func (m *Miner) Stop() {
	if m.status.Load().(bool) == false {
		return
	}

	m.status.Store(false)
	close(m.abortCh)
	m.wg.Wait()
	m.abortCh = make(chan struct{})
}

func (m *Miner) mine(threadID int, job *ngtypes.Block) {
	m.wg.Add(1)
	var target = new(big.Int).SetBytes(job.Header.Target)

	for {
		select {
		//case newJob := <-m.newJobCh:
		//	if !reflect.DeepEqual(b.Header, newJob.Header) {
		//		log.Debugf("Thread %d mining block@%d", threadID, newJob.Header.Height)
		//		b = newJob
		//		target = new(big.Int).SetBytes(b.Header.Target)
		//	}
		case <-m.abortCh:
			// Mining terminated, update stats and abort
			m.wg.Done()
			return
		default:
			if job == nil || !job.Header.IsUnsealing() {
				continue
			}

			// Compute the PoW value of this nonce
			nonce := make([]byte, 4)
			fastrand.Read(nonce)
			blob := job.Header.GetPoWBlob(nonce)
			hash := cryptonight.Sum(blob, 0)
			m.hashes++

			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				go m.found(threadID, job, nonce)
			}
		}
	}
}

func (m *Miner) found(t int, job *ngtypes.Block, nonce []byte) {
	// Correct nonce found, create a new header with it
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Stop()
	log.Infof("Thread %d found nonce %x", t, nonce)
	block, err := job.ToSealed(nonce)
	if err != nil {
		log.Panic(err)
	}

	m.foundCh <- block
}
