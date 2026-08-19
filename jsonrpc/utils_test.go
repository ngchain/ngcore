package jsonrpc

import (
	"math/big"
	"testing"
)

// TestNgToRaw pins the NG->raw conversion above the u64 ceiling: the old
// uint64(v * 1e18) overflowed for anything past ~18.4 NG and silently
// corrupted large transfers
func TestNgToRaw(t *testing.T) {
	for _, c := range []struct {
		ng   float64
		want string
	}{
		{0, "0"},
		{1.5, "1500000000000000000"},
		// 18.4 is NOT an exact float64; the conversion is faithful to the
		// caller's actual float64 value — and, the point, does not overflow
		{18.4, "18399999999999998578"},
		{100, "100000000000000000000"},         // would have overflowed
		{1000000, "1000000000000000000000000"}, // 1M NG
		{-5, "0"},                              // negatives clamp
	} {
		want, _ := new(big.Int).SetString(c.want, 10)
		if got := ngToRaw(c.ng); got.Cmp(want) != 0 {
			t.Fatalf("ngToRaw(%v) = %s, want %s", c.ng, got, want)
		}
	}
}
