package utils

import (
	"go.uber.org/atomic"
	"runtime"
)

type Locker struct {
	status *atomic.Bool
}

func NewLocker() *Locker {
	return &Locker{status: atomic.NewBool(false)}
}

func (l *Locker) Lock() {
	for !l.status.CAS(false, true) {
		runtime.Gosched()
	}
}

func (l *Locker) Unlock() {
	for !l.status.CAS(true, false) {
		runtime.Gosched()
	}
}

func (l *Locker) OnLock() bool {
	return l.status.Load()
}
