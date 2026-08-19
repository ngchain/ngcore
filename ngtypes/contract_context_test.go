package ngtypes

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestContractContextSetGetDel(t *testing.T) {
	ctx := NewContractContext()

	ctx.Set("b", []byte{2})
	ctx.Set("a", []byte{1})
	ctx.Set("c", []byte{3})

	if !bytes.Equal(ctx.Get("a"), []byte{1}) || !bytes.Equal(ctx.Get("c"), []byte{3}) {
		t.Fatal("Get must return what Set stored")
	}
	if ctx.Get("missing") != nil {
		t.Fatal("a missing key must return nil")
	}

	// the exported form stays sorted for deterministic encoding
	if len(ctx.Keys) != 3 || ctx.Keys[0] != "a" || ctx.Keys[1] != "b" || ctx.Keys[2] != "c" {
		t.Fatalf("keys not sorted: %v", ctx.Keys)
	}
	if !bytes.Equal(ctx.Values[0], []byte{1}) {
		t.Fatal("values must follow the sorted keys")
	}

	ctx.Del("b")
	if ctx.Get("b") != nil {
		t.Fatal("Del must remove the key")
	}
	if len(ctx.Keys) != 2 {
		t.Fatalf("keys after del: %v", ctx.Keys)
	}

	// overwrite
	ctx.Set("a", []byte{9})
	if !bytes.Equal(ctx.Get("a"), []byte{9}) {
		t.Fatal("Set must overwrite")
	}
}

// TestContractContextEnsureInit: a decoder-built context (exported
// fields only) still answers Get, even with a values shortfall
func TestContractContextEnsureInit(t *testing.T) {
	ctx := &ContractContext{
		Keys:   []string{"a", "b"},
		Values: [][]byte{{1}},
	}

	if !bytes.Equal(ctx.Get("a"), []byte{1}) {
		t.Fatal("decoder-built context must resolve keys")
	}
	if ctx.Get("b") != nil {
		t.Fatal("a key without a value resolves to nil")
	}
}

func TestContractContextClone(t *testing.T) {
	ctx := NewContractContext()
	ctx.Set("k", []byte{1, 2})

	clone := ctx.Clone()
	if eq, _ := ctx.Equals(clone); !eq {
		t.Fatal("a clone must equal its source")
	}

	// deep: mutating the clone leaves the source alone
	clone.Set("k", []byte{9})
	if !bytes.Equal(ctx.Get("k"), []byte{1, 2}) {
		t.Fatal("the clone must not share storage")
	}
	if eq, _ := ctx.Equals(clone); eq {
		t.Fatal("a mutated clone must differ")
	}
}

func TestContractContextEquals(t *testing.T) {
	a := NewContractContext()
	a.Set("k", []byte{1})

	// different sizes
	b := NewContractContext()
	if eq, _ := a.Equals(b); eq {
		t.Fatal("different sizes must not be equal")
	}

	// same key, different value
	b.Set("k", []byte{2})
	if eq, _ := a.Equals(b); eq {
		t.Fatal("different values must not be equal")
	}

	// same size, missing key
	c := NewContractContext()
	c.Set("other", []byte{1})
	if eq, _ := a.Equals(c); eq {
		t.Fatal("a missing key must not be equal")
	}
}

func TestContractContextJSONRoundTrip(t *testing.T) {
	ctx := NewContractContext()
	ctx.Set("alpha", []byte{0xde, 0xad})
	ctx.Set("beta", []byte{})

	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatal(err)
	}

	restored := NewContractContext()
	if err := json.Unmarshal(raw, restored); err != nil {
		t.Fatal(err)
	}

	if eq, _ := ctx.Equals(restored); !eq {
		t.Fatalf("json round trip lost data: %s", raw)
	}
}

func TestContractContextJSONErrors(t *testing.T) {
	ctx := NewContractContext()

	if err := json.Unmarshal([]byte(`[1,2]`), ctx); err == nil {
		t.Fatal("a non-map json must fail")
	}
	if err := json.Unmarshal([]byte(`{"k":"zz"}`), ctx); err == nil {
		t.Fatal("a non-hex value must fail")
	}
}
