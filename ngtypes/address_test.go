package ngtypes

import (
	"encoding/json"
	"testing"

	"github.com/ngchain/secp256k1"
)

// TestGenesisAddress pins the genesis address: the bs58 constant must
// decode to the all-zero 33-byte address and round-trip back — a bad
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
	key, err := secp256k1.GeneratePrivateKey()
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

// TestAddressLength pins the canonical length and both decode paths:
// modern strings must be exactly 33 bytes, and the legacy 35-byte
// genesis format must strip its 2-byte checksum down to the real
// public key instead of truncating it
func TestAddressLength(t *testing.T) {
	if AddressSize != len(Address{}) {
		t.Fatalf("AddressSize %d != len(Address) %d", AddressSize, len(Address{}))
	}

	// a legacy genesis string: 2-byte checksum + 33-byte pubkey
	const legacy = "QfUnsE4CNgnpVS4oC4WEYH8u7WWAs8AwMrFBknWWqGSYwBXU"

	if _, err := NewAddressFromBS58(legacy); err == nil {
		t.Fatal("the 35-byte legacy format must be rejected by the modern decoder")
	}

	addr, err := NewAddressFromLegacyBS58(legacy)
	if err != nil {
		t.Fatal(err)
	}
	// the payload is a compressed public key, so it starts 0x02 or 0x03
	if prefix := addr.Bytes()[0]; prefix != 0x02 && prefix != 0x03 {
		t.Fatalf("legacy decode kept the checksum: first byte %#02x", prefix)
	}

	if _, err := NewAddressFromLegacyBS58(GenesisAddressBase58); err == nil {
		t.Fatal("a 33-byte string must be rejected by the legacy decoder")
	}
}

// TestNewAddressFromMultiKeys: a single-key multi-address must equal
// the plain address of that key (the pubkey list used to stay empty)
func TestNewAddressFromMultiKeys(t *testing.T) {
	key, err := secp256k1.GeneratePrivateKey()
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
