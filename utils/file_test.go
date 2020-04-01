package utils

import "testing"

// Testing encryption and decryption
func TestFile(t *testing.T) {
	b := []byte("Hello")
	key := []byte("keyyyyyyyyyyyyyyyyyyy")
	t.Log(string(DataDecrypt(DataEncrypt(b, key), key)))
}
