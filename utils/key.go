package utils

import (
	"github.com/ngchain/go-schnorr"
	"github.com/ngchain/secp256k1"
)

// PublicKey2Bytes is a helper func to convert public key to the raw bytes
func PublicKey2Bytes(publicKey secp256k1.PublicKey) []byte {
	return schnorr.Marshal(secp256k1.S256(), publicKey.X, publicKey.Y)
}

// Bytes2PublicKey is a helper func to convert raw bytes to public key
func Bytes2PublicKey(data []byte) secp256k1.PublicKey {
	x, y := schnorr.Unmarshal(secp256k1.S256(), data)
	return *secp256k1.NewPublicKey(x, y)
}
