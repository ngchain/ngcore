package utils

import (
	"bytes"
	"testing"
)

func TestAES256GCMEncrypt(t *testing.T) {
	raw := []byte("hello")
	password := []byte("world")
	encrypted := AES256GCMEncrypt(raw, password)
	if !bytes.Equal(AES256GCMDecrypt(encrypted, password), raw) {
		t.Fail()
	}
}
