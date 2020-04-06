package utils

import (
	"bytes"
	"crypto/rand"
	"testing"
)

// Testing hex string to []byte
func TestBytes2Hex(t *testing.T) {
	b := make([]byte, 10000)
	_, err := rand.Read(b)
	if err != nil {
		t.Error(err)
	}
	s := Bytes2Hex(b)
	if !bytes.Equal(Hex2Bytes(s), b) {
		t.Error("Bytes2Hex or Hex2Bytes wrong")
	}
}
