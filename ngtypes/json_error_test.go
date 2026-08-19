package ngtypes

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBlockUnmarshalJSONErrors: every malformed field errors instead
// of yielding a half-built block
func TestBlockUnmarshalJSONErrors(t *testing.T) {
	valid := map[string]any{
		"network":       "ZERONET",
		"height":        1,
		"timestamp":     2,
		"prevBlockHash": strings.Repeat("00", HashSize),
		"txTrieHash":    strings.Repeat("00", HashSize),
		"subTrieHash":   strings.Repeat("00", HashSize),
		"difficulty":    "1",
		"nonce":         strings.Repeat("00", NonceSize),
		"txs":           []any{},
	}

	// the valid shape decodes
	raw, _ := json.Marshal(valid)
	var block FullBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatalf("the valid shape must decode: %v", err)
	}
	if block.Height != 1 || block.Network != ZERONET {
		t.Fatal("decoded fields are wrong")
	}

	corrupt := func(field, value string) []byte {
		m := make(map[string]any, len(valid))
		for k, v := range valid {
			m[k] = v
		}
		m[field] = value
		raw, _ := json.Marshal(m)
		return raw
	}

	cases := [][]byte{
		[]byte(`{`), // broken json
		corrupt("prevBlockHash", "zz"),
		corrupt("txTrieHash", "zz"),
		corrupt("subTrieHash", "zz"),
		corrupt("difficulty", "not-a-number"),
		corrupt("nonce", "zz"),
	}
	for i, raw := range cases {
		var b FullBlock
		if err := json.Unmarshal(raw, &b); err == nil {
			t.Fatalf("case %d must fail: %s", i, raw)
		}
	}
}

func TestTxUnmarshalJSONErrors(t *testing.T) {
	valid := `{"network":"ZERONET","type":3,"height":1,"to":"` + GenesisAddressBase58 +
		`","value":1,"fee":0,"extra":"","sign":""}`
	var tx FullTx
	if err := json.Unmarshal([]byte(valid), &tx); err != nil {
		t.Fatalf("the valid shape must decode: %v", err)
	}
	if tx.Type != TransactTx || tx.Height != 1 {
		t.Fatal("decoded fields are wrong")
	}

	cases := []string{
		`{`, // broken json
		strings.Replace(valid, `"extra":""`, `"extra":"zz"`, 1),
		strings.Replace(valid, `"sign":""`, `"sign":"zz"`, 1),
	}
	for i, raw := range cases {
		var tx FullTx
		if err := json.Unmarshal([]byte(raw), &tx); err == nil {
			t.Fatalf("case %d must fail: %s", i, raw)
		}
	}
}

func TestContractUnmarshalJSONErrors(t *testing.T) {
	valid := `{"owner":"` + GenesisAddressBase58 + `","source":"00","context":null}`
	var contract Contract
	if err := json.Unmarshal([]byte(valid), &contract); err != nil {
		t.Fatalf("the valid shape must decode: %v", err)
	}
	if contract.Context == nil {
		t.Fatal("a null context must be replaced with an empty one")
	}

	cases := []string{
		`{`,
		`{"owner":"` + GenesisAddressBase58 + `","source":"zz","context":null}`,
	}
	for i, raw := range cases {
		var c Contract
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			t.Fatalf("case %d must fail: %s", i, raw)
		}
	}
}

func TestContractActiveFlag(t *testing.T) {
	contract := &Contract{} // nil context on purpose
	if contract.IsActive() {
		t.Fatal("a nil-context contract is inactive")
	}

	contract.SetActive(true)
	if !contract.IsActive() {
		t.Fatal("SetActive(true) must activate")
	}

	contract.SetActive(false)
	if contract.IsActive() {
		t.Fatal("SetActive(false) must deactivate")
	}

	// SetActive on a nil context allocates one
	fresh := &Contract{}
	fresh.SetActive(true)
	if !fresh.IsActive() {
		t.Fatal("SetActive must allocate a missing context")
	}
}

func TestContractEquals(t *testing.T) {
	key, _ := GenerateKey()
	owner := NewAddress(key)

	a := NewContract(owner, []byte("mod"), nil)
	b := NewContract(owner, []byte("mod"), nil)
	if eq, _ := a.Equals(b); !eq {
		t.Fatal("identical contracts must be equal")
	}

	if eq, _ := a.Equals(NewContract(Address{1}, []byte("mod"), nil)); eq {
		t.Fatal("differing owners must not be equal")
	}
	if eq, _ := a.Equals(NewContract(owner, []byte("other"), nil)); eq {
		t.Fatal("differing sources must not be equal")
	}

	c := NewContract(owner, []byte("mod"), nil)
	c.Context.Set("k", []byte{1})
	if eq, _ := a.Equals(c); eq {
		t.Fatal("differing contexts must not be equal")
	}
}
