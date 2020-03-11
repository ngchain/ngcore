package keyManager

import (
	"reflect"
	"testing"
)

func TestKeyMgr_ReadLocalKey(t *testing.T) {
	mgr := NewKeyManager("ngtest.key", "test")
	privKey := mgr.CreateLocalKey()

	mgr2 := NewKeyManager("ngtest.key", "test")
	privKey2 := mgr2.ReadLocalKey()

	if !reflect.DeepEqual(privKey, privKey2) {
		t.Log(privKey)
		t.Log(privKey2)
		t.Fail()
	}
}
