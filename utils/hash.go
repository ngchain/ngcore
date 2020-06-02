package utils

import "golang.org/x/crypto/sha3"

// Sha3Sum256 is a helper func to calc & return the sha3 sum256 []byte hash
func Sha3Sum256(b []byte) []byte {
	hash := sha3.Sum256(b)

	return hash[:]
}
