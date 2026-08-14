package ngtypes_test

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

func TestApplyEdits(t *testing.T) {
	text := []byte("(module\n  (func $a)\n  (func $b)\n)\n")

	// replace one line and append another via two hunks
	got, err := (&ngtypes.CommitExtra{Hunks: []ngtypes.Hunk{
		{Pos: 10, Del: []byte("(func $a)\n"), Ins: []byte("(func $a (result i32) (i32.const 1))\n")},
		{Pos: 32, Del: nil, Ins: []byte("  (memory 1)\n")},
	}}).Apply(text)
	if err != nil {
		t.Fatal(err)
	}
	want := "(module\n  (func $a (result i32) (i32.const 1))\n  (func $b)\n  (memory 1)\n)\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// mismatching Del must fail
	if _, err := (&ngtypes.CommitExtra{Hunks: []ngtypes.Hunk{
		{Pos: 10, Del: []byte("(func $x)\n"), Ins: []byte("y")},
	}}).Apply(text); err != ngtypes.ErrHunkMismatch {
		t.Fatalf("want ErrHunkMismatch, got %v", err)
	}

	// overlapping hunks must fail
	if _, err := (&ngtypes.CommitExtra{Hunks: []ngtypes.Hunk{
		{Pos: 10, Del: []byte("(func $a)\n"), Ins: []byte("x")},
		{Pos: 12, Del: nil, Ins: []byte("y")},
	}}).Apply(text); err != ngtypes.ErrHunkOverlap {
		t.Fatalf("want ErrHunkOverlap, got %v", err)
	}

	// out of bound must fail
	if _, err := (&ngtypes.CommitExtra{Hunks: []ngtypes.Hunk{
		{Pos: uint64(len(text)), Del: []byte("z"), Ins: nil},
	}}).Apply(text); err != ngtypes.ErrHunkOutOfBound {
		t.Fatalf("want ErrHunkOutOfBound, got %v", err)
	}

	// starting a contract from empty text
	got, err = (&ngtypes.CommitExtra{Hunks: []ngtypes.Hunk{
		{Pos: 0, Del: nil, Ins: []byte("(module)")},
	}}).Apply(nil)
	if err != nil || string(got) != "(module)" {
		t.Fatalf("empty-start failed: %q, %v", got, err)
	}
}

func TestCommitExtraHashedShape(t *testing.T) {
	text := []byte("(module\n  (func $aaaa (result i32) (i32.const 1))\n  (func $bbbb (result i32) (i32.const 2))\n  (func $cccc (result i32) (i32.const 3))\n)\n")

	// a big deletion switches to the hashed shape: no Del bytes on chain
	hunks := ngtypes.DiffHunks(text, []byte("(module)\n"))
	extra := ngtypes.NewCommitExtra(text, hunks)
	if len(extra.BaseHash) == 0 {
		t.Fatal("big deletion should use the hashed shape")
	}
	for _, h := range extra.Hunks {
		if len(h.Del) != 0 {
			t.Fatal("hashed shape must not carry Del bytes")
		}
	}

	got, err := extra.Apply(text)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "(module)\n" {
		t.Fatalf("hashed apply mismatch: %q", got)
	}

	// the same patch against a modified base must fail on the hash
	if _, err := extra.Apply(append([]byte("x"), text...)); err != ngtypes.ErrBaseMismatch {
		t.Fatalf("want ErrBaseMismatch, got %v", err)
	}

	// a tiny patch stays in the content shape
	small := ngtypes.NewCommitExtra(text, []ngtypes.Hunk{{Pos: 0, Del: []byte("("), Ins: []byte("(")}})
	if len(small.BaseHash) != 0 {
		t.Fatal("tiny patch should stay in the content shape")
	}
}

func TestCommitExtraEncodeDecode(t *testing.T) {
	text := []byte("(module\n  (func $a)\n)\n")

	// tiny patch: raw encoding, roundtrip
	extra := ngtypes.NewCommitExtra(text, []ngtypes.Hunk{{Pos: 2, Del: []byte("od"), Ins: []byte("OD")}})
	enc, err := extra.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dec, err := ngtypes.DecodeCommitExtra(enc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dec.Apply(text)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "(mODule\n  (func $a)\n)\n" {
		t.Fatalf("roundtrip apply mismatch: %q", got)
	}

	// a large repetitive deploy must get compressed
	bigIns := bytes.Repeat([]byte("  (func $pad (result i32) (i32.const 42))\n"), 200)
	deploy := ngtypes.NewCommitExtra(nil, []ngtypes.Hunk{{Pos: 0, Ins: bigIns}})
	encBig, err := deploy.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encBig) >= len(bigIns)/2 {
		t.Fatalf("deploy did not compress: %d bytes for %d bytes of text", len(encBig), len(bigIns))
	}
	decBig, err := ngtypes.DecodeCommitExtra(encBig)
	if err != nil {
		t.Fatal(err)
	}
	gotBig, err := decBig.Apply(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBig, bigIns) {
		t.Fatal("compressed roundtrip mismatch")
	}

	// garbage must be rejected
	for _, bad := range [][]byte{nil, {0x00}, {0x99, 0x01, 0x02}, {0x01, 0xff, 0xff}} {
		if _, err := ngtypes.DecodeCommitExtra(bad); err == nil {
			t.Fatalf("decode(%x) should fail", bad)
		}
	}
}

func TestDiffHunksRoundtrip(t *testing.T) {
	oldText := []byte(`(module
  (import "coin" "transfer" (func $transfer (param i64 i64) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $transfer (i64.const 1) (i64.const 10)))))
`)

	cases := [][]byte{
		// single line change
		bytes.Replace(oldText, []byte("i64.const 10"), []byte("i64.const 25"), 1),
		// add a new import + a new func
		[]byte(`(module
  (import "coin" "transfer" (func $transfer (param i64 i64) (result i32)))
  (import "log" "debug" (func $debug (param i32 i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $transfer (i64.const 1) (i64.const 10)))))
`),
		// delete the data line
		bytes.Replace(oldText, []byte("  (data (i32.const 0) \"keyval\")\n"), nil, 1),
		// full rewrite
		[]byte("(module)\n"),
		// empty target
		{},
	}

	for i, newText := range cases {
		// the full wire pipeline: diff -> shape pick -> encode -> decode -> apply
		enc, err := ngtypes.NewCommitExtra(oldText, ngtypes.DiffHunks(oldText, newText)).Encode()
		if err != nil {
			t.Fatalf("case %d: encode failed: %v", i, err)
		}
		dec, err := ngtypes.DecodeCommitExtra(enc)
		if err != nil {
			t.Fatalf("case %d: decode failed: %v", i, err)
		}
		got, err := dec.Apply(oldText)
		if err != nil {
			t.Fatalf("case %d: apply failed: %v", i, err)
		}
		if !bytes.Equal(got, newText) {
			t.Fatalf("case %d: roundtrip mismatch:\ngot  %q\nwant %q", i, got, newText)
		}
	}

	// identical texts must produce no hunks
	if hunks := ngtypes.DiffHunks(oldText, oldText); len(hunks) != 0 {
		t.Fatalf("identical texts produced %d hunks", len(hunks))
	}

	// a small change must produce a patch much smaller than the text
	small := bytes.Replace(oldText, []byte("i64.const 10"), []byte("i64.const 25"), 1)
	hunks := ngtypes.DiffHunks(oldText, small)
	patchSize := 0
	for _, h := range hunks {
		patchSize += len(h.Del) + len(h.Ins)
	}
	if patchSize > 16 {
		t.Fatalf("small edit produced a big patch: %d bytes vs text %d bytes", patchSize, len(oldText))
	}
}
