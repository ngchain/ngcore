package jsonrpc

import (
	"math/big"
	"testing"
)

// TestNgAmount pins the exact decimal-string NG parser: no float touches
// a money path, so every representable amount converts exactly and the
// old u64/float ceilings are gone
func TestNgAmount(t *testing.T) {
	for _, c := range []struct {
		ng   string
		want string
	}{
		{"", "0"},
		{"0", "0"},
		{"1.5", "1500000000000000000"},
		{"18.4", "18400000000000000000"},         // exact — impossible with float64
		{"100", "100000000000000000000"},         // past the old u64 ceiling
		{"1000000", "1000000000000000000000000"}, // 1M NG
		{"0.000000000000000001", "1"},            // 1 raw unit (18 decimals)
		{"123456789.987654321123456789", "123456789987654321123456789"}, // full precision
	} {
		want, _ := new(big.Int).SetString(c.want, 10)
		got, err := ngAmount(c.ng)
		if err != nil {
			t.Fatalf("ngAmount(%q): %v", c.ng, err)
		}
		if got.Cmp(want) != 0 {
			t.Fatalf("ngAmount(%q) = %s, want %s", c.ng, got, want)
		}
	}

	// rejected inputs: sub-raw precision, negatives, garbage
	for _, bad := range []string{"0.0000000000000000001", "-5", "abc", "1.2.3"} {
		if _, err := ngAmount(bad); err == nil {
			t.Fatalf("ngAmount(%q) accepted, want error", bad)
		}
	}
}
