package utils_test

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/utils"
)

func TestAES256GCMEncrypt(t *testing.T) {
	t.Parallel()

	raw := []byte("hello")
	password := []byte("world")
	encrypted := utils.AES256GCMEncrypt(raw, password)

	if !bytes.Equal(utils.AES256GCMDecrypt(encrypted, password), raw) {
		t.Fail()
	}
}

func TestAES256GCMRoundTripVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      []byte
		password []byte
	}{
		{"empty raw", []byte{}, []byte("pass")},
		{"empty password", []byte("payload"), []byte{}},
		{"both empty", []byte{}, []byte{}},
		{"binary raw", []byte{0x00, 0xff, 0x10, 0x80}, []byte("s3cret")},
		{"long raw", bytes.Repeat([]byte{0xab}, 4096), []byte("long password with spaces and 中文")},
	}

	for _, c := range cases {
		encrypted := utils.AES256GCMEncrypt(c.raw, c.password)

		if bytes.Contains(encrypted, c.raw) && len(c.raw) > 0 {
			t.Errorf("%s: ciphertext contains plaintext", c.name)
		}

		decrypted := utils.AES256GCMDecrypt(encrypted, c.password)
		if !bytes.Equal(decrypted, c.raw) {
			t.Errorf("%s: round trip failed: got %x, want %x", c.name, decrypted, c.raw)
		}
	}
}

func TestAES256GCMEncryptIsRandomized(t *testing.T) {
	t.Parallel()

	raw := []byte("same input")
	password := []byte("same password")

	a := utils.AES256GCMEncrypt(raw, password)
	b := utils.AES256GCMEncrypt(raw, password)

	// a fresh random nonce is drawn per call, so ciphertexts must differ
	if bytes.Equal(a, b) {
		t.Error("two encryptions of the same input produced identical ciphertext")
	}
}

func TestAES256GCMDecryptWrongPasswordPanics(t *testing.T) {
	t.Parallel()

	encrypted := utils.AES256GCMEncrypt([]byte("data"), []byte("right"))

	mustPanic(t, "AES256GCMDecrypt(wrong password)", func() {
		utils.AES256GCMDecrypt(encrypted, []byte("wrong"))
	})
}

func TestAES256GCMDecryptTamperedPanics(t *testing.T) {
	t.Parallel()

	encrypted := utils.AES256GCMEncrypt([]byte("data"), []byte("pass"))
	encrypted[len(encrypted)-1] ^= 0xff

	mustPanic(t, "AES256GCMDecrypt(tampered)", func() {
		utils.AES256GCMDecrypt(encrypted, []byte("pass"))
	})
}

func TestAES256GCMDecryptTooShortPanics(t *testing.T) {
	t.Parallel()

	// shorter than the 12-byte GCM nonce
	mustPanic(t, "AES256GCMDecrypt(too short)", func() {
		utils.AES256GCMDecrypt([]byte{0x01, 0x02, 0x03}, []byte("pass"))
	})
}
