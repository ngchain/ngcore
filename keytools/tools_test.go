package keytools_test

import (
	"reflect"
	"testing"

	"github.com/ngchain/ngcore/keytools"
)

func TestKeyMgr_ReadLocalKey(t *testing.T) {
	privKey := keytools.CreateLocalKey("ngtest.key", "test")

	privKey2 := keytools.ReadLocalKey("ngtest.key", "test")

	if !reflect.DeepEqual(privKey, privKey2) {
		t.Log(privKey)
		t.Log(privKey2)
		t.Fail()
	}
}
