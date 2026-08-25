package utils

import "lukechampine.com/blake3"

// Hash256 is the chain's 256-bit hash primitive: BLAKE3.
//
// A cryptographic hash is already post-quantum — Grover's algorithm gives
// only a quadratic speedup, so a 256-bit digest keeps ~128-bit preimage
// security against a quantum attacker (matching the ML-DSA-44 security
// level). BLAKE3 preserves that guarantee while being substantially faster
// and more parallel than the Keccak-256 it replaces.
func Hash256(b []byte) []byte {
	h := blake3.Sum256(b)

	return h[:]
}
