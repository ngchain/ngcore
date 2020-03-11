package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
)

func ECDSAPublicKey2Bytes(pk ecdsa.PublicKey) []byte {
	return elliptic.Marshal(elliptic.P256(), pk.X, pk.Y)
}

func Bytes2ECDSAPublicKey(data []byte) ecdsa.PublicKey {
	x, y := elliptic.Unmarshal(elliptic.P256(), data)
	return ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
}
