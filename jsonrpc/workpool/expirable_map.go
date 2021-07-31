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

func (m *ExpirableMap) Len() int {
	return len(m.m)
}

func (m *ExpirableMap) Put(k string, v interface{}) {
	m.l.Lock()
	it, ok := m.m[k]
	if !ok {
		it = &Entry{Value: v}
		m.m[k] = it
	}
	it.Timestamp = time.Now().Unix()
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
