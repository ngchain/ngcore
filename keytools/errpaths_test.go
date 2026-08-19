package keytools_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
)

// With an empty $HOME, os.UserHomeDir fails on unix/macOS, so the default
// path helpers must panic. These tests cannot run in parallel because they
// mutate the process environment via t.Setenv.
func TestGetDefaultFolderNoHomePanics(t *testing.T) {
	t.Setenv("HOME", "")

	mustPanic(t, "GetDefaultFolder(no HOME)", func() {
		keytools.GetDefaultFolder()
	})
}

func TestGetDefaultFileNoHomePanics(t *testing.T) {
	t.Setenv("HOME", "")

	mustPanic(t, "GetDefaultFile(no HOME)", func() {
		keytools.GetDefaultFile()
	})
}

// When the default folder path already exists as a regular file, the Mkdir
// inside the empty-filename branch fails and the functions panic.
func TestReadLocalKeyMkdirFailsPanics(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// occupy the ".ngkeys" name with a regular file so Mkdir fails
	if err := os.WriteFile(filepath.Join(home, ".ngkeys"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "ReadLocalKey(mkdir fails)", func() {
		keytools.ReadLocalKey("", "pass")
	})
}

func TestCreateLocalKeyMkdirFailsPanics(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, ".ngkeys"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "CreateLocalKey(mkdir fails)", func() {
		keytools.CreateLocalKey("", "pass")
	})
}

func TestRecoverLocalKeyMkdirFailsPanics(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, ".ngkeys"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	orig := keytools.NewLocalKey()

	mustPanic(t, "RecoverLocalKey(mkdir fails)", func() {
		keytools.RecoverLocalKey("", "pass", base58.FastBase58Encoding(orig.Serialize()))
	})
}

// When the target keyfile path cannot be opened for writing (its parent
// directory does not exist), the OpenFile call panics.
func TestCreateLocalKeyOpenFileFailsPanics(t *testing.T) {
	t.Parallel()

	// parent directory "nope" does not exist under the temp dir
	bad := filepath.Join(t.TempDir(), "nope", "ngcore.key")

	mustPanic(t, "CreateLocalKeyWithScheme(open fails)", func() {
		keytools.CreateLocalKeyWithScheme(bad, "pass", ngtypes.SchemeSecp256k1)
	})
}

func TestRecoverLocalKeyOpenFileFailsPanics(t *testing.T) {
	t.Parallel()

	bad := filepath.Join(t.TempDir(), "nope", "ngcore.key")
	orig := keytools.NewLocalKey()

	mustPanic(t, "RecoverLocalKey(open fails)", func() {
		keytools.RecoverLocalKey(bad, "pass", base58.FastBase58Encoding(orig.Serialize()))
	})
}

// GetP2PKey with an empty HOME cannot resolve the default folder and panics.
func TestGetP2PKeyNoHomePanics(t *testing.T) {
	t.Setenv("HOME", "")

	mustPanic(t, "GetP2PKey(no HOME)", func() {
		keytools.GetP2PKey("")
	})
}

// GetP2PKey with the default ".ngkeys" name occupied by a regular file must
// panic on the failing Mkdir.
func TestGetP2PKeyMkdirFailsPanics(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, ".ngkeys"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "GetP2PKey(mkdir fails)", func() {
		keytools.GetP2PKey("")
	})
}

// GetP2PKey pointing at a non-existent nested path fails to create the file
// and panics on OpenFile.
func TestGetP2PKeyOpenFileFailsPanics(t *testing.T) {
	t.Parallel()

	bad := filepath.Join(t.TempDir(), "nope", "ngp2p.key")

	mustPanic(t, "GetP2PKey(open fails)", func() {
		keytools.GetP2PKey(bad)
	})
}
