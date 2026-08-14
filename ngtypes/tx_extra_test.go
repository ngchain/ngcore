package ngtypes_test

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

func TestApplyEdits(t *testing.T) {
	text := []byte("(module\n  (func $a)\n  (func $b)\n)\n")

	// replace one line and append another via two hunks
	got, err := ngtypes.ApplyEdits(text, []ngtypes.Hunk{
		{Pos: 10, Del: []byte("(func $a)\n"), Ins: []byte("(func $a (result i32) (i32.const 1))\n")},
		{Pos: 32, Del: nil, Ins: []byte("  (memory 1)\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "(module\n  (func $a (result i32) (i32.const 1))\n  (func $b)\n  (memory 1)\n)\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// mismatching Del must fail
	if _, err := ngtypes.ApplyEdits(text, []ngtypes.Hunk{
		{Pos: 10, Del: []byte("(func $x)\n"), Ins: []byte("y")},
	}); err != ngtypes.ErrHunkMismatch {
		t.Fatalf("want ErrHunkMismatch, got %v", err)
	}

	// overlapping hunks must fail
	if _, err := ngtypes.ApplyEdits(text, []ngtypes.Hunk{
		{Pos: 10, Del: []byte("(func $a)\n"), Ins: []byte("x")},
		{Pos: 12, Del: nil, Ins: []byte("y")},
	}); err != ngtypes.ErrHunkOverlap {
		t.Fatalf("want ErrHunkOverlap, got %v", err)
	}

	// out of bound must fail
	if _, err := ngtypes.ApplyEdits(text, []ngtypes.Hunk{
		{Pos: uint64(len(text)), Del: []byte("z"), Ins: nil},
	}); err != ngtypes.ErrHunkOutOfBound {
		t.Fatalf("want ErrHunkOutOfBound, got %v", err)
	}

	// starting a contract from empty text
	got, err = ngtypes.ApplyEdits(nil, []ngtypes.Hunk{
		{Pos: 0, Del: nil, Ins: []byte("(module)")},
	})
	if err != nil || string(got) != "(module)" {
		t.Fatalf("empty-start failed: %q, %v", got, err)
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
		hunks := ngtypes.DiffHunks(oldText, newText)
		got, err := ngtypes.ApplyEdits(oldText, hunks)
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
