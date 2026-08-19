package workpool

import (
	"sync"
	"time"
)

type Entry struct {
	Timestamp int64
	Value     interface{}
}

type ExpirableMap struct {
	m map[string]*Entry
	l sync.RWMutex
}

func NewExpirableMap(l int, expire func(time.Time, *Entry) bool) (m *ExpirableMap) {
	m = &ExpirableMap{m: make(map[string]*Entry, l)}
	go func() {
		for now := range time.Tick(time.Second) {
			m.l.Lock()
			for k, entry := range m.m {
				if expire(now, entry) {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

func (m *ExpirableMap) Put(k string, v interface{}) {
	m.l.Lock()
	// overwrite the value on an existing key: the old code kept the FIRST
	// value and only refreshed the timestamp, so Put silently dropped
	// updates
	m.m[k] = &Entry{Value: v, Timestamp: time.Now().Unix()}
	m.l.Unlock()
}

func (m *ExpirableMap) Get(k string) (v interface{}, ok bool) {
	m.l.RLock()
	var it *Entry
	if it, ok = m.m[k]; ok {
		v = it.Value
		it.Timestamp = time.Now().Unix() // update the last use time
		m.m[k] = it
	}
	m.l.RUnlock()
	return
}
