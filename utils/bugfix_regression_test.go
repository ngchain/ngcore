package utils_test

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/utils"
)

// TestAESDecryptShortInput: a ciphertext shorter than the nonce must give
// the controlled "shorter than the nonce" panic, not a raw slice-bounds
// out-of-range (the two are distinguishable by message)
func TestAESDecryptShortInput(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("decrypting a too-short ciphertext should panic")
		}
		if s, ok := r.(string); !ok || s != "ciphertext shorter than the nonce" {
			t.Fatalf("panic = %v, want the controlled length-guard message", r)
		}
	}()
	utils.AES256GCMDecrypt([]byte("short"), []byte("pw")) // 5 bytes < 12-byte nonce
}

// TestLockerUnlockIdempotent: Unlock must return immediately even on an
// unlocked Locker (the old CAS spun forever). A goroutine + timeout would
// be needed to catch a hang; here we just call it and Lock again to prove
// state is sane.
func TestLockerUnlockOnUnlocked(t *testing.T) {
	l := utils.NewLocker()
	done := make(chan struct{})
	go func() {
		l.Unlock() // must not block on an already-unlocked Locker
		l.Lock()   // still acquirable
		l.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Unlock on an unlocked Locker hung (old spin bug)")
	}
	if l.IsActive() {
		t.Fatal("Locker still active after Unlock")
	}
}
