package utils

import (
	"runtime"

	"go.uber.org/atomic"
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

func (l *Locker) IsLocked() bool {
	return l.status.Load()
}
