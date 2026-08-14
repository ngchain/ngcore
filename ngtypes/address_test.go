package ngtypes

import (
	"encoding/json"
	"testing"
)

// TestGenesisAddress pins the genesis address: the bs58 constant must
// decode to the all-zero 32-byte address and round-trip back — a bad
// constant used to be silently swallowed into the zero value
func TestGenesisAddress(t *testing.T) {
	if GenesisAddress != (Address{}) {
		t.Fatalf("genesis address must be all-zero, got %x", GenesisAddress.Bytes())
	}

	if got := GenesisAddress.BS58(); got != GenesisAddressBase58 {
		t.Fatalf("genesis address round trip: %q != %q", got, GenesisAddressBase58)
	}
}

// TestAddressJSONRoundTrip: an address survives json encode/decode
// inside a struct field (the decode used to hit a value receiver and
// silently return the zero address)
func TestAddressJSONRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := NewAddress(key)

	raw, err := json.Marshal(struct{ Owner Address }{addr})
	if err != nil {
		t.Fatal(err)
	}

	var out struct{ Owner Address }
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if !out.Owner.Equals(addr) {
		t.Fatalf("round trip lost the address: %s != %s", out.Owner, addr)
	}
}

// TestAddressLength pins the canonical length: every address is the
// 32-byte keccak hash of its keyset descriptor
func TestAddressLength(t *testing.T) {
	if AddressSize != len(Address{}) {
		t.Fatalf("AddressSize %d != len(Address) %d", AddressSize, len(Address{}))
	}

	// a legacy pre-hash string (2-byte checksum + 33-byte pubkey) must
	// be rejected: the genesis sheet was converted once, offline
	const legacy = "QfUnsE4CNgnpVS4oC4WEYH8u7WWAs8AwMrFBknWWqGSYwBXU"
	if _, err := NewAddressFromBS58(legacy); err == nil {
		t.Fatal("a 35-byte legacy string must be rejected")
	}
}

// TestNewAddressFromMultiKeys: a single-key multi-address must equal
// the plain address of that key (the pubkey list used to stay empty)
func TestNewAddressFromMultiKeys(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	multi, err := NewAddressFromMultiKeys(key)
	if err != nil {
		t.Fatal(err)
	}

	if !multi.Equals(NewAddress(key)) {
		t.Fatalf("single-key multi address %s != plain address %s", multi, NewAddress(key))
	}
}
