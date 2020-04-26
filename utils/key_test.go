package utils_test

import (
	"reflect"
	"testing"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/utils"
)

func TestPublicKey2Bytes(t *testing.T) {
	pk, _ := secp256k1.GeneratePrivateKey()
	publicKey1 := *pk.PubKey()
	raw := utils.PublicKey2Bytes(publicKey1)
	publicKey2 := utils.Bytes2PublicKey(raw)
	if !reflect.DeepEqual(publicKey1, publicKey2) {
		t.Fail()
	}
}
