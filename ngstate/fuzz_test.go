package ngstate

import (
	"testing"
)

// FuzzLoadContractWasm hammers the wasm loader — the CONSENSUS-path
// validator runs on attacker-chosen contract bytecode at activation.
// It must reject malformed input with an error, never panic or hang
func FuzzLoadContractWasm(f *testing.F) {
	f.Add(mustWat(kvWat))
	f.Add(mustWat(u256Wat))
	f.Add(mustWat(multiEntryWat))
	f.Add(mustWat(cryptoWat))
	f.Add([]byte{0x00, 0x61, 0x73, 0x6d})       // bare magic
	f.Add([]byte{0x00, 0x61, 0x73, 0x6d, 0xff}) // magic + garbage
	f.Add([]byte("not wasm at all"))

	f.Fuzz(func(t *testing.T, source []byte) {
		_, _ = LoadContractWasm(source) // must not panic
	})
}
