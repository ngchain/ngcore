package keytools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic but got none", name)
		}
	}()

	fn()
}

func TestKeyMgr_ReadLocalKey(t *testing.T) {
	t.Parallel()

	filename := filepath.Join(t.TempDir(), "ngtest.key")

	privKey := keytools.CreateLocalKey(filename, "test")
	privKey2 := keytools.ReadLocalKey(filename, "test")

	if !bytes.Equal(privKey.Serialize(), privKey2.Serialize()) {
		t.Log(privKey)
		t.Log(privKey2)
		t.Fail()
	}

	pk := keytools.RecoverLocalKey(filename, "test", base58.FastBase58Encoding(privKey.Serialize()))
	if !bytes.Equal(pk.Serialize(), privKey.Serialize()) {
		t.Log(privKey)
		t.Log(pk)
		t.Fail()
	}
}

func TestNewLocalKey(t *testing.T) {
	t.Parallel()

	key := keytools.NewLocalKey()
	if key == nil {
		t.Fatal("NewLocalKey returned nil")
	}

	if key.Scheme != ngtypes.SchemeDefault {
		t.Errorf("NewLocalKey scheme = %#x, want default %#x", key.Scheme, ngtypes.SchemeDefault)
	}
}

func TestCreateLocalKeyWithSchemes(t *testing.T) {
	t.Parallel()

	schemes := []ngtypes.SigScheme{
		ngtypes.SchemeSecp256k1,
		ngtypes.SchemeFNDSA512,
		ngtypes.SchemeMLDSA44,
		ngtypes.SchemeSLHDSA128,
	}

	for _, scheme := range schemes {
		filename := filepath.Join(t.TempDir(), "scheme.key")

		key := keytools.CreateLocalKeyWithScheme(filename, "pass", scheme)
		if key.Scheme != scheme {
			t.Errorf("created key scheme = %#x, want %#x", key.Scheme, scheme)
		}

		// the on-disk file must be encrypted, not the raw serialized key
		raw, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read keyfile: %v", err)
		}

		if bytes.Contains(raw, key.Serialize()) {
			t.Errorf("scheme %#x: keyfile stores the key unencrypted", scheme)
		}

		read := keytools.ReadLocalKey(filename, "pass")
		if !bytes.Equal(read.Serialize(), key.Serialize()) {
			t.Errorf("scheme %#x: read back mismatch", scheme)
		}

		if read.Scheme != scheme {
			t.Errorf("scheme %#x: read back scheme = %#x", scheme, read.Scheme)
		}

		if !bytes.Equal(read.PublicBytes(), key.PublicBytes()) {
			t.Errorf("scheme %#x: public key mismatch after reload", scheme)
		}
	}
}

func TestReadLocalKeyCreatesWhenMissing(t *testing.T) {
	t.Parallel()

	filename := filepath.Join(t.TempDir(), "fresh.key")

	key := keytools.ReadLocalKey(filename, "pass")
	if key == nil {
		t.Fatal("ReadLocalKey returned nil")
	}

	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("keyfile was not created: %v", err)
	}

	again := keytools.ReadLocalKey(filename, "pass")
	if !bytes.Equal(again.Serialize(), key.Serialize()) {
		t.Error("second read returned a different key")
	}
}

func TestReadLocalKeyWrongPasswordPanics(t *testing.T) {
	t.Parallel()

	filename := filepath.Join(t.TempDir(), "wrongpass.key")
	keytools.CreateLocalKey(filename, "right")

	mustPanic(t, "ReadLocalKey(wrong password)", func() {
		keytools.ReadLocalKey(filename, "wrong")
	})
}

func TestReadLocalKeyCorruptFilePanics(t *testing.T) {
	t.Parallel()

	filename := filepath.Join(t.TempDir(), "corrupt.key")
	if err := os.WriteFile(filename, []byte("this is not a valid encrypted keyfile"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "ReadLocalKey(corrupt file)", func() {
		keytools.ReadLocalKey(filename, "pass")
	})
}

func TestReadLocalKeyInvalidKeyMaterialPanics(t *testing.T) {
	t.Parallel()

	// decryptable with the right password, but the plaintext is not a
	// valid serialized key (wrong length), so ParsePrivateKey must fail
	filename := filepath.Join(t.TempDir(), "badkey.key")
	encrypted := utils.AES256GCMEncrypt([]byte("too short"), []byte("pass"))

	if err := os.WriteFile(filename, encrypted, 0o600); err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "ReadLocalKey(invalid key material)", func() {
		keytools.ReadLocalKey(filename, "pass")
	})
}

func TestRecoverLocalKeyErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	mustPanic(t, "RecoverLocalKey(invalid base58)", func() {
		keytools.RecoverLocalKey(filepath.Join(dir, "a.key"), "pass", "0OIl-not-base58")
	})

	mustPanic(t, "RecoverLocalKey(wrong length)", func() {
		keytools.RecoverLocalKey(filepath.Join(dir, "b.key"), "pass", base58.FastBase58Encoding([]byte{1, 2, 3}))
	})
}

func TestDefaultPathHelpers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	folder := keytools.GetDefaultFolder()
	if folder != filepath.Join(home, ".ngkeys") {
		t.Errorf("GetDefaultFolder = %s", folder)
	}

	file := keytools.GetDefaultFile()
	if file != filepath.Join(home, ".ngkeys", "ngcore.key") {
		t.Errorf("GetDefaultFile = %s", file)
	}
}

func TestEmptyFilenameUsesDefaultLocation(t *testing.T) {
	// each subtest gets a fresh fake HOME so the .ngkeys mkdir branch
	// is exercised every time; t.Setenv keeps this off the real home dir
	t.Run("CreateLocalKey", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		key := keytools.CreateLocalKey("", "pass")

		read := keytools.ReadLocalKey(filepath.Join(home, ".ngkeys", "ngcore.key"), "pass")
		if !bytes.Equal(read.Serialize(), key.Serialize()) {
			t.Error("default-location keyfile mismatch")
		}
	})

	t.Run("ReadLocalKey", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		key := keytools.ReadLocalKey("", "pass")

		if _, err := os.Stat(filepath.Join(home, ".ngkeys", "ngcore.key")); err != nil {
			t.Fatalf("default keyfile was not created: %v", err)
		}

		again := keytools.ReadLocalKey("", "pass")
		if !bytes.Equal(again.Serialize(), key.Serialize()) {
			t.Error("re-reading the default keyfile returned a different key")
		}
	})

	t.Run("RecoverLocalKey", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		orig := keytools.NewLocalKey()
		key := keytools.RecoverLocalKey("", "pass", base58.FastBase58Encoding(orig.Serialize()))

		if !bytes.Equal(key.Serialize(), orig.Serialize()) {
			t.Error("recovered key mismatch")
		}

		read := keytools.ReadLocalKey(filepath.Join(home, ".ngkeys", "ngcore.key"), "pass")
		if !bytes.Equal(read.Serialize(), orig.Serialize()) {
			t.Error("recovered keyfile mismatch")
		}
	})
}

func TestPrintKeysAndAddress(t *testing.T) {
	t.Parallel()

	// just make sure it renders every scheme without panicking
	schemes := []ngtypes.SigScheme{
		ngtypes.SchemeSecp256k1,
		ngtypes.SchemeFNDSA512,
		ngtypes.SchemeMLDSA44,
		ngtypes.SchemeSLHDSA128,
	}

	for _, scheme := range schemes {
		key, err := ngtypes.GenerateSchemeKey(scheme)
		if err != nil {
			t.Fatalf("scheme %#x: %v", scheme, err)
		}

		keytools.PrintKeysAndAddress(key)
	}
}
