package ngstate

import (
	"testing"
)

// FuzzCompileContract hammers the wat compiler — a self-written parser
// on the CONSENSUS path (activation compiles attacker-chosen source
// inside block validation). It must reject garbage with an error,
// never panic or hang
func FuzzCompileContract(f *testing.F) {
	f.Add([]byte(kvWat))
	f.Add([]byte(burnWat))
	f.Add([]byte(logWat))
	f.Add([]byte(u256Wat))
	f.Add([]byte(kvScanWat))
	f.Add([]byte(multiEntryWat))
	f.Add([]byte(cryptoWat))
	f.Add([]byte(usdtTokenWat))
	f.Add([]byte(`(module`))
	f.Add([]byte(`(module (func (export "a") unreachable))`))
	f.Add([]byte{0x00, 0x61, 0x73, 0x6d})

	f.Fuzz(func(t *testing.T, source []byte) {
		_, _ = CompileContract(source) // must not panic
	})
}
