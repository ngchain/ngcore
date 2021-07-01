package utils_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/ngchain/ngcore/utils"
)

// Testing hex string to []byte.
func TestBytes2Hex(t *testing.T) {
	b := make([]byte, 10000)
	_, err := rand.Read(b)
	if err != nil {
		t.Error(err)
	}
	s := utils.Bytes2Hex(b)
	if !bytes.Equal(utils.Hex2Bytes(s), b) {
		t.Error("Bytes2Hex or Hex2Bytes wrong")
	}
}
