package keytools_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
)

// TestKeyFilePermissions: a saved private key must be 0600 and its
// directory 0700 — not the old 0666/0777
func TestKeyFilePermissions(t *testing.T) {
	dir := t.TempDir()
	kf := filepath.Join(dir, "sub", "ngcore.key")
	_ = os.MkdirAll(filepath.Dir(kf), 0o700)

	keytools.CreateLocalKeyWithScheme(kf, "pw", ngtypes.SchemeSecp256k1)

	fi, err := os.Stat(kf)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key file perm = %o, want 600", perm)
	}
}
