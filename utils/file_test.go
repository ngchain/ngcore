package utils

import "testing"

func TestFile(t *testing.T) {
	b := []byte("Hello")
	key := []byte("keyyyyyyyyyyyyyyyyyyy")
	t.Log(string(DataDecrypt(DataEncrypt(b, key), key)))
}
