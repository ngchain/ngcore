// Package workpool_test pins the tiny expirable work-template store the
// mining rpc hands templates through: singleton behavior, put/get
// round-trips, last-use refresh and the background expiry sweeper.
package workpool_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/jsonrpc/workpool"
)

func TestGetWorkerPoolSingleton(t *testing.T) {
	first := workpool.GetWorkerPool()
	if first == nil {
		t.Fatal("GetWorkerPool returned nil")
	}

	second := workpool.GetWorkerPool()
	if first != second {
		t.Fatal("GetWorkerPool must always return the same pool")
	}
}

func TestWorkPoolPutGet(t *testing.T) {
	pool := workpool.GetWorkerPool()

	if _, err := pool.Get("no-such-work"); err == nil {
		t.Fatal("Get on a missing key must fail")
	} else if err != workpool.ErrBlockNotExists {
		t.Fatalf("Get on a missing key = %v, want ErrBlockNotExists", err)
	}

	type work struct{ ID uint64 }
	pool.Put("42", &work{ID: 42})

	got, err := pool.Get("42")
	if err != nil {
		t.Fatalf("Get after Put: %v", err)
	}
	if got.(*work).ID != 42 {
		t.Fatalf("Get returned %+v, want ID 42", got)
	}

	// overwriting an existing key keeps the entry alive and readable
	pool.Put("42", &work{ID: 43})
	got, err = pool.Get("42")
	if err != nil {
		t.Fatalf("Get after second Put: %v", err)
	}

	// the pool's 10-minute expiry must NOT collect a fresh entry even
	// after the sweeper has run (it ticks every second)
	time.Sleep(1200 * time.Millisecond)
	if _, err := pool.Get("42"); err != nil {
		t.Fatalf("a fresh entry was expired: %v", err)
	}
	_ = got
}

func TestExpirableMapExpires(t *testing.T) {
	// an always-expire predicate: the sweeper must collect every entry
	// on its next tick
	m := workpool.NewExpirableMap(0, func(time.Time, *workpool.Entry) bool {
		return true
	})

	m.Put("gone", "soon")
	if _, ok := m.Get("gone"); !ok {
		t.Fatal("the entry must be readable before the sweeper runs")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := m.Get("gone"); !ok {
			break // collected
		}
		if time.Now().After(deadline) {
			t.Fatal("the sweeper never collected an always-expired entry")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestExpirableMapRefreshOnUse(t *testing.T) {
	// expire anything untouched for over a second: Get refreshes the
	// timestamp, so a busy entry survives while an idle one dies
	m := workpool.NewExpirableMap(0, func(now time.Time, e *workpool.Entry) bool {
		return now.Unix()-e.Timestamp > 1
	})

	m.Put("busy", 1)
	m.Put("idle", 2)

	for i := 0; i < 30; i++ {
		if _, ok := m.Get("busy"); !ok {
			t.Fatal("a constantly used entry must not expire")
		}
		time.Sleep(100 * time.Millisecond)
	}

	if _, ok := m.Get("idle"); ok {
		t.Fatal("an idle entry must expire")
	}

	// Put on an existing key overwrites the value (and refreshes the ts)
	m.Put("busy", 3)
	if v, ok := m.Get("busy"); !ok || v.(int) != 3 {
		t.Fatalf("Get(busy) = %v, %v; want the overwritten value 3", v, ok)
	}
}
