package utils

import (
	"reflect"
	"testing"

	"github.com/ngchain/secp256k1"
)

func TestPublicKey2Bytes(t *testing.T) {
	pk, _ := secp256k1.GeneratePrivateKey()
	publicKey1 := pk.PubKey()
	raw := PublicKey2Bytes(*publicKey1)
	publicKey2 := Bytes2PublicKey(raw)
	if !reflect.DeepEqual(publicKey1, publicKey2) {
		t.Fail()
	}
}
