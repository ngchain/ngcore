package utils

import (
	"github.com/btcsuite/btcd/btcec/v2"
)

// PublicKey2Bytes is a helper func to convert public key to the **compressed** raw bytes.
func PublicKey2Bytes(publicKey *btcec.PublicKey) []byte {
	return publicKey.SerializeCompressed()
}

// Bytes2PublicKey is a helper func to convert **compressed** raw bytes to
// public key. Returns nil when the bytes are not a valid curve point.
func Bytes2PublicKey(data []byte) *btcec.PublicKey {
	publicKey, err := btcec.ParsePubKey(data)
	if err != nil {
		return nil
	}

	return publicKey
}
