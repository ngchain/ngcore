package utils

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestBytes2String(t *testing.T) {
	b := make([]byte, 100)
	_, err := rand.Read(b)
	if err != nil {
		t.Error(err)
	}
	s := Bytes2String(b)
	if bytes.Compare(String2Bytes(s), b) != 0 {
		t.Error("Bytes2String or String2Bytes wrong")
	}
}

func TestBytes2Hex(t *testing.T) {
	b := make([]byte, 10000)
	_, err := rand.Read(b)
	if err != nil {
		t.Error(err)
	}
	s := Bytes2Hex(b)
	if bytes.Compare(Hex2Bytes(s), b) != 0 {
		t.Error("Bytes2Hex or Hex2Bytes wrong")
	}
}
