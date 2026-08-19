package keytools_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ngchain/ngcore/keytools"
)

func TestGetP2PKeyCreatesAndReloads(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ngp2p.key")

	created := keytools.GetP2PKey(path)
	if created == nil {
		t.Fatal("GetP2PKey returned nil")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("p2p keyfile was not created: %v", err)
	}

	reloaded := keytools.GetP2PKey(path)
	if !created.Equals(reloaded) {
		t.Error("reloaded p2p key differs from the created one")
	}
}

func TestGetP2PKeyDefaultLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	key := keytools.GetP2PKey("")
	if key == nil {
		t.Fatal("GetP2PKey returned nil")
	}

	path := filepath.Join(home, ".ngkeys", "ngp2p.key")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default p2p keyfile was not created: %v", err)
	}

	// second call must read the existing file back, not regenerate
	again := keytools.GetP2PKey("")
	if !key.Equals(again) {
		t.Error("second GetP2PKey call returned a different key")
	}
}

func TestGetP2PKeyCorruptFilePanics(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "corrupt-p2p.key")
	if err := os.WriteFile(path, []byte("not a marshaled libp2p key"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "GetP2PKey(corrupt file)", func() {
		keytools.GetP2PKey(path)
	})
}

func TestGetP2PKeyUnreadablePathPanics(t *testing.T) {
	t.Parallel()

	// a directory passes the existence check but cannot be read as a
	// keyfile, driving the readKeyFromFile error path
	dir := t.TempDir()

	mustPanic(t, "GetP2PKey(directory)", func() {
		keytools.GetP2PKey(dir)
	})
}
