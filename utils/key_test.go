package utils_test

import (
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/ngchain/ngcore/utils"
)

func TestKeys(t *testing.T) {
	pk, err := btcec.NewPrivateKey()
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

	if utils.Bytes2PublicKey([]byte{0xff, 0x01}) != nil {
		t.Fatal("garbage bytes must not parse into a public key")
	}
}
