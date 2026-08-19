package defaults

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// NOTE: only ZERONET is exercised here. GetGenesisBlock(MAINNET) panics
// ("not ready for mainnet"), and ngtypes.GetGenesisBlock caches a
// singleton which IGNORES the network parameter after the first call,
// so cross-network name separation cannot be asserted within one
// process (see the suspected-bug report)
func TestProtocolAndTopicNames(t *testing.T) {
	network := ngtypes.ZERONET
	genesisHash := hex.EncodeToString(ngtypes.GetGenesisBlock(network).GetHash())

	if len(genesisHash) != ngtypes.HashSize*2 {
		t.Fatalf("unexpected genesis hash %q", genesisHash)
	}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"wired", GetWiredProtocol(network), "/ngp2p/wired/" + genesisHash + "/0.0.1"},
		{"dht", GetDHTProtocolExtension(network), "/ngp2p/dht/" + genesisHash + "/0.0.1"},
		{"block topic", GetBroadcastBlockTopic(network), "/ngp2p/broadcast/block/" + genesisHash + "/0.0.1"},
		{"tx topic", GetBroadcastTxTopic(network), "/ngp2p/broadcast/tx/" + genesisHash + "/0.0.1"},
	}

	seen := make(map[string]string)

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}

		if !strings.Contains(c.got, genesisHash) {
			t.Errorf("%s does not embed the genesis hash", c.name)
		}

		// every purpose must get its own namespace
		if prev, dup := seen[c.got]; dup {
			t.Errorf("%s collides with %s", c.name, prev)
		}
		seen[c.got] = c.name
	}
}

func TestMaxBlocks(t *testing.T) {
	if MaxBlocks <= 0 {
		t.Fatalf("MaxBlocks must be positive, got %d", MaxBlocks)
	}
}
