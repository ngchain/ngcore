package workpool

import (
	"errors"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

type WorkPool struct {
	m *ExpirableMap
}

var workPool *WorkPool

func init() {
	workPool = &WorkPool{
		NewExpirableMap(0, func(t time.Time, _ *Entry) bool {
			now := time.Now().Unix()
			if now-t.Unix() > 60*10 { // expire in 10 min
				return true
			}
			return false
		}),
	}
}

func GetWorkerPool() *WorkPool {
	return workPool
}

var ErrBlockNotExists = errors.New("no such block in the work pool")
var ErrValNotBlock = errors.New("the value in pool is not a block template")

func (wp *WorkPool) Get(k string) (*ngtypes.Block, error) {
	iBlock, ok := wp.m.Get(k)
	if !ok {
		return nil, ErrBlockNotExists
	}

	block, ok := iBlock.(*ngtypes.Block)
	if !ok {
		return nil, ErrValNotBlock
	}

	return block, nil
}

func (wp *WorkPool) Put(k string, v *ngtypes.Block) {
	wp.m.Put(k, v)
}
