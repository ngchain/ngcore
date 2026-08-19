package ngtypes

import (
	"testing"

	"github.com/pkg/errors"
)

// TestGenesisPerNetwork: the singleton must key on network, not just on
// first-call (the old cache froze to the first caller's network)
func TestGenesisPerNetwork(t *testing.T) {
	z := GetGenesisBlock(ZERONET)
	te := GetGenesisBlock(TESTNET)
	if z.Network != ZERONET || te.Network != TESTNET {
		t.Fatalf("genesis leaked network: zero=%v test=%v", z.Network, te.Network)
	}
	// and calling zero again returns a ZERONET block, not the last one
	if GetGenesisBlock(ZERONET).Network != ZERONET {
		t.Fatal("GetGenesisBlock(ZERONET) returned a non-zero network block")
	}
	if GetGenesisSheet(ZERONET).Network != ZERONET || GetGenesisSheet(TESTNET).Network != TESTNET {
		t.Fatal("GetGenesisSheet leaked network across calls")
	}
}

// TestParseNetwork: never panics on unknown input (unlike GetNetwork)
func TestParseNetwork(t *testing.T) {
	for name, want := range map[string]Network{"ZERONET": ZERONET, "TESTNET": TESTNET, "MAINNET": MAINNET} {
		if got, err := ParseNetwork(name); err != nil || got != want {
			t.Fatalf("ParseNetwork(%q) = %v,%v", name, got, err)
		}
	}
	if _, err := ParseNetwork("FOO"); !errors.Is(err, ErrNetworkInvalid) {
		t.Fatalf("ParseNetwork(FOO) err = %v, want ErrNetworkInvalid", err)
	}
}

// TestTxJSONMalformedNoPanic: attacker-controlled JSON must error, not
// panic the node (unknown network name; negative value/fee)
func TestTxJSONMalformedNoPanic(t *testing.T) {
	for _, body := range []string{
		`{"network":"FOO","type":2,"height":1,"to":"11111111111111111111111111111111 impossible","value":"1","fee":"0","extra":"","sign":""}`,
		`{"network":"ZERONET","type":2,"height":1,"value":-1,"fee":0,"extra":"","sign":""}`,
		`{"network":"ZERONET","type":2,"height":1,"value":1,"fee":-5,"extra":"","sign":""}`,
	} {
		var tx FullTx
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("UnmarshalJSON panicked on %q: %v", body, r)
				}
			}()
			if err := tx.UnmarshalJSON([]byte(body)); err == nil {
				t.Fatalf("UnmarshalJSON accepted malformed %q", body)
			}
		}()
	}
}

// TestBlockJSONUnknownNetworkNoPanic: same for a block
func TestBlockJSONUnknownNetworkNoPanic(t *testing.T) {
	var b FullBlock
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("block UnmarshalJSON panicked: %v", r)
		}
	}()
	err := b.UnmarshalJSON([]byte(`{"network":"NOPE","height":0,"timestamp":0,"prevBlockHash":"","txTrieHash":"","subTrieHash":"","difficulty":"1","nonce":"","txs":[]}`))
	if err == nil {
		t.Fatal("block accepted unknown network")
	}
}
