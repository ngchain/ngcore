package utils

import "golang.org/x/crypto/sha3"

// Sha3Sum256 is a helper func to calc & return the sha3 sum256 []byte hash.
func Sha3Sum256(b []byte) []byte {
	hash := sha3.Sum256(b)

	return hash[:]
}

// KeccakSum256 hashes with the original Keccak-256 (the pre-NIST
// padding, as used by the ethereum ecosystem), keeping addresses
// friendly to existing crypto tooling
func KeccakSum256(b []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(b)

	return h.Sum(nil)
}
