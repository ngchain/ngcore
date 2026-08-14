package utils

import "golang.org/x/crypto/sha3"

// KeccakSum256 hashes with the original Keccak-256 (the pre-NIST
// padding, as used by the ethereum ecosystem), keeping addresses
// friendly to existing crypto tooling
func KeccakSum256(b []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(b)

	return h.Sum(nil)
}
