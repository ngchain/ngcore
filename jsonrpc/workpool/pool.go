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

func GetWorkerPool() *WorkPool {
	if workPool == nil {
		workPool = &WorkPool{
			NewExpirableMap(0, func(t time.Time, _ *Entry) bool {
				now := time.Now().Unix()
				return now-t.Unix() > 60*10 // expire in 10 min
			}),
		}
	}

	return workPool
}

var (
	ErrBlockNotExists = errors.New("no such block in the work pool")
	ErrValNotBlock    = errors.New("the value in pool is not a block template")
)

func (wp *WorkPool) Get(k string) (any, error) {
	iBlock, ok := wp.m.Get(k)
	if !ok {
		return nil, ErrBlockNotExists
	}

	block, ok := iBlock.(*ngtypes.FullBlock)
	if !ok {
		return nil, ErrValNotBlock
	}

	return block, nil
}

func (wp *WorkPool) Put(k string, v any) {
	wp.m.Put(k, v)
}
