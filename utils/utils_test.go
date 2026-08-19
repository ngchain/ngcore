package utils_test

import (
	"bytes"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/ngchain/ngcore/utils"
)

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic but got none", name)
		}
	}()

	fn()
}

func TestPackUint64LE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n    uint64
		want []byte
	}{
		{0, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{1, []byte{1, 0, 0, 0, 0, 0, 0, 0}},
		{0x0102030405060708, []byte{8, 7, 6, 5, 4, 3, 2, 1}},
		{^uint64(0), []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}

	for _, tt := range tests {
		if got := utils.PackUint64LE(tt.n); !bytes.Equal(got, tt.want) {
			t.Errorf("PackUint64LE(%d) = %x, want %x", tt.n, got, tt.want)
		}
	}
}

func TestReverseBytes(t *testing.T) {
	t.Parallel()

	orig := []byte{1, 2, 3, 4, 5}
	got := utils.ReverseBytes(orig)

	if !bytes.Equal(got, []byte{5, 4, 3, 2, 1}) {
		t.Errorf("ReverseBytes = %v", got)
	}

	// the input must not be mutated
	if !bytes.Equal(orig, []byte{1, 2, 3, 4, 5}) {
		t.Errorf("ReverseBytes mutated its input: %v", orig)
	}

	if got := utils.ReverseBytes(nil); len(got) != 0 {
		t.Errorf("ReverseBytes(nil) = %v, want empty", got)
	}

	if got := utils.ReverseBytes([]byte{7}); !bytes.Equal(got, []byte{7}) {
		t.Errorf("ReverseBytes single = %v", got)
	}

	// even length
	if got := utils.ReverseBytes([]byte{1, 2}); !bytes.Equal(got, []byte{2, 1}) {
		t.Errorf("ReverseBytes even = %v", got)
	}
}

func TestHexRLPEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	// string round trip
	encoded := utils.HexRLPEncode("hello world")

	var s string
	if err := utils.HexRLPDecode(encoded, &s); err != nil {
		t.Fatalf("HexRLPDecode string: %v", err)
	}

	if s != "hello world" {
		t.Errorf("round trip string = %q", s)
	}

	// bytes round trip
	raw := []byte{0xde, 0xad, 0xbe, 0xef}
	encoded = utils.HexRLPEncode(raw)

	var b []byte
	if err := utils.HexRLPDecode(encoded, &b); err != nil {
		t.Fatalf("HexRLPDecode bytes: %v", err)
	}

	if !bytes.Equal(b, raw) {
		t.Errorf("round trip bytes = %x, want %x", b, raw)
	}

	// struct round trip
	type pair struct {
		A uint64
		B []byte
	}

	in := pair{A: 42, B: []byte("ng")}
	encoded = utils.HexRLPEncode(&in)

	var out pair
	if err := utils.HexRLPDecode(encoded, &out); err != nil {
		t.Fatalf("HexRLPDecode struct: %v", err)
	}

	if out.A != in.A || !bytes.Equal(out.B, in.B) {
		t.Errorf("round trip struct = %+v, want %+v", out, in)
	}
}

func TestHexRLPEncodeKnownVector(t *testing.T) {
	t.Parallel()

	// canonical RLP: "dog" -> 0x83 'd' 'o' 'g'
	if got := utils.HexRLPEncode("dog"); got != "83646f67" {
		t.Errorf("HexRLPEncode(dog) = %s, want 83646f67", got)
	}
}

func TestHexRLPDecodeErrors(t *testing.T) {
	t.Parallel()

	var s string

	// invalid hex characters
	if err := utils.HexRLPDecode("zz", &s); err == nil {
		t.Error("HexRLPDecode should fail on invalid hex")
	}

	// valid hex, invalid/truncated RLP payload
	if err := utils.HexRLPDecode("83", &s); err == nil {
		t.Error("HexRLPDecode should fail on truncated RLP")
	}

	// valid RLP but wrong target type (list into string)
	if err := utils.HexRLPDecode("c0", &s); err == nil {
		t.Error("HexRLPDecode should fail on type mismatch")
	}
}

func TestHexRLPEncodePanicsOnUnsupportedType(t *testing.T) {
	t.Parallel()

	mustPanic(t, "HexRLPEncode(chan)", func() {
		_ = utils.HexRLPEncode(make(chan int))
	})
}

func TestMinUint64(t *testing.T) {
	t.Parallel()

	if got := utils.MinUint64(1, 2); got != 1 {
		t.Errorf("MinUint64(1, 2) = %d", got)
	}

	if got := utils.MinUint64(2, 1); got != 1 {
		t.Errorf("MinUint64(2, 1) = %d", got)
	}

	if got := utils.MinUint64(7, 7); got != 7 {
		t.Errorf("MinUint64(7, 7) = %d", got)
	}

	if got := utils.MinUint64(0, ^uint64(0)); got != 0 {
		t.Errorf("MinUint64(0, max) = %d", got)
	}
}

func TestKeccakSum256(t *testing.T) {
	t.Parallel()

	// legacy Keccak-256 (pre-NIST padding) well-known vectors
	tests := []struct {
		in   []byte
		want string
	}{
		{[]byte{}, "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		{[]byte("hello"), "1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"},
		{[]byte("abc"), "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"},
	}

	for _, tt := range tests {
		got := hex.EncodeToString(utils.KeccakSum256(tt.in))
		if got != tt.want {
			t.Errorf("KeccakSum256(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
		N    uint64 `json:"n"`
	}

	in := payload{Name: "ngcore", N: 42}

	raw, err := utils.JSON.Marshal(in)
	if err != nil {
		t.Fatalf("JSON.Marshal: %v", err)
	}

	if string(raw) != `{"name":"ngcore","n":42}` {
		t.Errorf("JSON.Marshal = %s", raw)
	}

	var out payload
	if err := utils.JSON.Unmarshal(raw, &out); err != nil {
		t.Fatalf("JSON.Unmarshal: %v", err)
	}

	if out != in {
		t.Errorf("JSON round trip = %+v, want %+v", out, in)
	}

	if err := utils.JSON.Unmarshal([]byte("{"), &out); err == nil {
		t.Error("JSON.Unmarshal should fail on invalid input")
	}
}

func TestLocker(t *testing.T) {
	t.Parallel()

	l := utils.NewLocker()

	if l.IsActive() {
		t.Error("new locker should not be active")
	}

	l.Lock()

	if !l.IsActive() {
		t.Error("locker should be active after Lock")
	}

	l.Unlock()

	if l.IsActive() {
		t.Error("locker should not be active after Unlock")
	}
}

func TestLockerConcurrent(t *testing.T) {
	t.Parallel()

	l := utils.NewLocker()

	var wg sync.WaitGroup

	counter := 0

	for range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100 {
				l.Lock()
				counter++
				l.Unlock()
			}
		}()
	}

	wg.Wait()

	if counter != 800 {
		t.Errorf("counter = %d, want 800", counter)
	}

	if l.IsActive() {
		t.Error("locker should be released after all goroutines finish")
	}
}
