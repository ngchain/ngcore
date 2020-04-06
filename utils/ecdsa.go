package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
)

// ECDSAPublicKey2Bytes is a helper func to convert public key to the raw bytes
func ECDSAPublicKey2Bytes(pk ecdsa.PublicKey) []byte {
	return elliptic.Marshal(elliptic.P256(), pk.X, pk.Y)
}

// Bytes2ECDSAPublicKey is a helper func to convert raw bytes to public key
func Bytes2ECDSAPublicKey(data []byte) ecdsa.PublicKey {
	x, y := elliptic.Unmarshal(elliptic.P256(), data)
	return ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
}
