package utils_test

import (
	"reflect"
	"testing"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/utils"
)

func TestKeys(t *testing.T) {
	pk, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		panic(err)
	}
	publicKey1 := pk.PubKey()
	raw := utils.PublicKey2Bytes(publicKey1)
	t.Log(len(raw))
	publicKey2 := utils.Bytes2PublicKey(raw)
	if !reflect.DeepEqual(publicKey1, publicKey2) {
		t.Fail()
	}

	msgHash := utils.Sha3Sum256([]byte("msg"))
	hash := [32]byte{}
	copy(hash[:], msgHash)
}
