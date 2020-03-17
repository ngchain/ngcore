package ngtypes

import (
	"bytes"
	"testing"
)

func TestVault_CalculateHash(t *testing.T) {
	v := GetGenesisBlock()
	r, _ := v.Marshal()
	prev := r
	for {
		_v := v
		__v := _v
		r, _ := __v.CalculateHash()
		prev = r
		if bytes.Compare(prev, r) != 0 {
			t.Fail()
		}
	}
}
