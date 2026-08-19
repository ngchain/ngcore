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
	for !l.status.CompareAndSwap(false, true) {
		runtime.Gosched()
	}
}

func (l *Locker) Unlock() {
	// releasing is unconditional: the old CAS(true,false) spun FOREVER if
	// Unlock was ever called on an unlocked Locker — a misuse that
	// deadlocked the caller instead of surfacing the bug
	l.status.Store(false)
}

func (l *Locker) IsActive() bool {
	return l.status.Load()
}
