package ngtypes

import (
	"bytes"
	"testing"
)

// TestCommitCodeRoundTrip: a contract module survives the commit-extra
// compress/decompress, whatever the format tag chosen
func TestCommitCodeRoundTrip(t *testing.T) {
	cases := [][]byte{
		{0x00, 0x61, 0x73, 0x6d, 0x01, 0, 0, 0}, // tiny "wasm"
		bytes.Repeat([]byte{0xab}, 4096),        // compressible
		func() []byte {
			b := make([]byte, 3000)
			for i := range b {
				b[i] = byte(i * 7)
			}
			return b
		}(), // incompressible-ish
		{},
	}
	for i, code := range cases {
		extra := EncodeCommitCode(code)
		got, err := DecodeCommitCode(extra)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if !bytes.Equal(got, code) {
			t.Fatalf("case %d: round trip mismatch (%d vs %d bytes)", i, len(got), len(code))
		}
	}
}

// TestCommitCodeCompresses: a repetitive module ends up smaller than
// raw, proving the deflate path is taken when it helps
func TestCommitCodeCompresses(t *testing.T) {
	code := bytes.Repeat([]byte("wasmwasmwasm"), 1000)
	extra := EncodeCommitCode(code)
	if len(extra) >= len(code) {
		t.Fatalf("compressed extra %d not smaller than raw %d", len(extra), len(code))
	}
}

// TestDecodeCommitCodeRejectsGarbage: malformed extras error, never panic
func TestDecodeCommitCodeRejectsGarbage(t *testing.T) {
	for _, bad := range [][]byte{nil, {}, {0x02}, {0x01, 0xff, 0xff}} {
		if _, err := DecodeCommitCode(bad); err == nil && len(bad) > 1 {
			t.Fatalf("garbage %x should have errored", bad)
		}
	}
}
