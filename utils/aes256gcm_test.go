package utils_test

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/utils"
)

func TestAES256GCMEncrypt(t *testing.T) {
	t.Parallel()

	raw := []byte("hello")
	password := []byte("world")
	encrypted := utils.AES256GCMEncrypt(raw, password)

	if !bytes.Equal(utils.AES256GCMDecrypt(encrypted, password), raw) {
		t.Fail()
	}
}
