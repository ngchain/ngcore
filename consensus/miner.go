package consensus

import (
	"github.com/NebulousLabs/fastrand"
	"github.com/ngin-network/cryptonight-go"
	"github.com/ngin-network/ngcore/ngtypes"
	"math/big"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Miner
type Miner struct {
	threadNum int
	hashes    int64

	newJobCh chan *ngtypes.Block
	abortCh  chan struct{}
	foundCh  chan *ngtypes.Block
	mu       sync.Mutex
}

func NewMiner(threadNum int, foundCh chan *ngtypes.Block) *Miner {
	runtime.GOMAXPROCS(runtime.NumCPU())

	newJobCh := make(chan *ngtypes.Block, 1)
	m := &Miner{
		threadNum: threadNum,
		newJobCh:  newJobCh,
		abortCh:   make(chan struct{}),
		foundCh:   foundCh,
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

func (m *Miner) start(initJob *ngtypes.Block) {
	for threadID := 0; threadID < m.threadNum; threadID++ {
		go m.mine(threadID, initJob)
	}
}

func (m *Miner) setJob(b *ngtypes.Block) {
	m.newJobCh <- b
}

func (m *Miner) mine(threadID int, initJob *ngtypes.Block) {
	var b = initJob
	var target = new(big.Int).SetBytes(b.Header.Target)

	for {
		select {
		case newJob := <-m.newJobCh:
			if !reflect.DeepEqual(b.Header, newJob.Header) {
				log.Debugf("Thread %d mining block@%d", threadID, newJob.Header.Height)
				b = newJob
				target = new(big.Int).SetBytes(b.Header.Target)
			}
		case <-m.abortCh:
			// Mining terminated, update stats and abort
			break
		default:
			if b == nil || !b.Header.IsUnsealing() {
				continue
			}

			// Compute the PoW value of this nonce
			nonce := make([]byte, 4)
			fastrand.Read(nonce)
			blob := b.Header.GetPoWBlob(nonce)
			hash := cryptonight.Sum(blob, 0)
			m.hashes++

			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				// Correct nonce found, create a new header with it
				log.Infof("Thread %d found nonce %x", threadID, nonce)
				block, err := b.ToSealed(nonce)
				if err != nil {
					log.Panic(err)
				}
				m.foundCh <- block
			}
		}
	}
}
