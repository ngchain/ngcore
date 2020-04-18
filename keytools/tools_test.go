package keytools

import (
	"reflect"
	"testing"
)

func TestKeyMgr_ReadLocalKey(t *testing.T) {
	privKey := CreateLocalKey("ngtest.key", "test")

	privKey2 := ReadLocalKey("ngtest.key", "test")

	if !reflect.DeepEqual(privKey, privKey2) {
		t.Log(privKey)
		t.Log(privKey2)
		t.Fail()
	}
}
