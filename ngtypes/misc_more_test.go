package ngtypes

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"
)

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s must panic", name)
		}
	}()
	fn()
}

// ---- calldata ----

func TestCallDataRoundTrip(t *testing.T) {
	// the default entry (empty or the canonical EntryOnTx) with no args is
	// NO calldata
	if len(EncodeCallData("", nil)) != 0 || len(EncodeCallData(EntryOnTx, nil)) != 0 {
		t.Fatal("a bare default-entry call must encode empty")
	}

	// the canonical default-entry name canonicalizes to the empty method;
	// note plain "main" is now an ORDINARY method name (no longer reserved)
	method, args, err := DecodeCallData(EncodeCallData(EntryOnTx, []byte{1, 2}))
	if err != nil || method != "" || !bytes.Equal(args, []byte{1, 2}) {
		t.Fatalf("default-entry call decode = %q, %x, %v", method, args, err)
	}
	if m, _, _ := DecodeCallData(EncodeCallData("main", []byte{1, 2})); m != "main" {
		t.Fatalf("plain \"main\" must stay an ordinary method, got %q", m)
	}

	// a named export with args
	method, args, err = DecodeCallData(EncodeCallData("transfer", []byte{9}))
	if err != nil || method != "transfer" || !bytes.Equal(args, []byte{9}) {
		t.Fatalf("named call decode = %q, %x, %v", method, args, err)
	}

	// a named export without args
	method, args, err = DecodeCallData(EncodeCallData("tick", nil))
	if err != nil || method != "tick" || len(args) != 0 {
		t.Fatalf("argless call decode = %q, %x, %v", method, args, err)
	}

	// garbage extras error, and so does the empty extra
	if _, _, err := DecodeCallData([]byte{}); err == nil {
		t.Fatal("the empty extra must not decode")
	}
	if _, _, err := DecodeCallData([]byte{0xff, 0x00}); err == nil {
		t.Fatal("garbage must not decode")
	}
}

// ---- sheet ----

func TestSheet(t *testing.T) {
	key, _ := GenerateKey()
	addr := NewAddress(key)

	balances := []*Balance{{Address: addr, Amount: big.NewInt(7)}}
	contracts := []*Contract{NewContract(addr, []byte("mod"), nil)}
	keys := []*RegisteredKey{{Address: addr, Entry: append([]byte{byte(key.Scheme)}, key.PublicBytes()...)}}

	sheet := NewSheet(ZERONET, 9, make([]byte, HashSize), balances, contracts, keys)
	if sheet.Network != ZERONET || sheet.Height != 9 ||
		len(sheet.Balances) != 1 || len(sheet.Contracts) != 1 || len(sheet.Keys) != 1 {
		t.Fatalf("NewSheet lost fields: %+v", sheet)
	}

	genesis := GetGenesisSheet(ZERONET)
	if genesis == nil || genesis.Height != 0 || len(genesis.Balances) != 0 {
		t.Fatal("the genesis sheet must be empty at height 0")
	}
	if GetGenesisSheet(ZERONET) != genesis {
		t.Fatal("the genesis sheet is a singleton")
	}
}

// ---- tx trie ----

func TestTxTrie(t *testing.T) {
	key, _ := GenerateKey()
	tx1 := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(1), big.NewInt(0), nil)
	tx2 := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(2), big.NewInt(0), nil)
	stranger := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(3), big.NewInt(0), nil)

	trie := NewTxTrie([]*FullTx{tx2, tx1})
	if !bytes.Equal(trie[0].GetHash(), func() []byte {
		a, b := tx1.GetHash(), tx2.GetHash()
		if bytes.Compare(a, b) < 0 {
			return a
		}
		return b
	}()) {
		t.Fatal("NewTxTrie must sort by hash")
	}

	if !trie.Contains(tx1) || !trie.Contains(tx2) {
		t.Fatal("Contains must find member txs")
	}
	if trie.Contains(stranger) {
		t.Fatal("Contains must not find strangers")
	}

	root := trie.TrieRoot()
	if len(root) != HashSize || bytes.Equal(root, make([]byte, HashSize)) {
		t.Fatal("a non-empty trie has a non-zero root")
	}

	empty := TxTrie{}
	if !bytes.Equal(empty.TrieRoot(), make([]byte, HashSize)) {
		t.Fatal("the empty trie root is all-zero")
	}
}

// ---- network ----

func TestNetwork(t *testing.T) {
	for _, c := range []struct {
		name string
		net  Network
	}{
		{"ZERONET", ZERONET},
		{"TESTNET", TESTNET},
		{"MAINNET", MAINNET},
	} {
		if GetNetwork(c.name) != c.net {
			t.Fatalf("GetNetwork(%q) is wrong", c.name)
		}
		if c.net.String() != c.name {
			t.Fatalf("%v.String() is wrong", c.net)
		}
	}

	mustPanic(t, "GetNetwork(bogus)", func() { GetNetwork("BOGUSNET") })
	mustPanic(t, "Network(99).String()", func() { _ = Network(99).String() })
}

// ---- defaults ----

func TestDefaults(t *testing.T) {
	if !bytes.Equal(GetEmptyHash(), make([]byte, HashSize)) {
		t.Fatal("the empty hash is all-zero")
	}

	if GetMatureHeight(MatureHeight-1) != 0 {
		t.Fatal("below one round nothing is mature")
	}
	if GetMatureHeight(MatureHeight) != MatureHeight {
		t.Fatal("the mature height snaps to the round")
	}
	if GetMatureHeight(2*MatureHeight+5) != 2*MatureHeight {
		t.Fatal("the mature height rounds down")
	}

	// the genesis constants exist for the launched networks…
	for _, net := range AvailableNetworks {
		if len(GetGenesisGenerateTxSignature(net)) == 0 ||
			len(GetGenesisBlockNonce(net)) != NonceSize ||
			GetGenesisTimestamp(net) == 0 {
			t.Fatalf("missing genesis constants for %s", net)
		}
	}

	// …and refuse to exist for mainnet or unknown networks
	mustPanic(t, "mainnet sig", func() { GetGenesisGenerateTxSignature(MAINNET) })
	mustPanic(t, "unknown sig", func() { GetGenesisGenerateTxSignature(Network(99)) })
	mustPanic(t, "mainnet nonce", func() { GetGenesisBlockNonce(MAINNET) })
	mustPanic(t, "unknown nonce", func() { GetGenesisBlockNonce(Network(99)) })
	mustPanic(t, "mainnet timestamp", func() { GetGenesisTimestamp(MAINNET) })
	mustPanic(t, "unknown timestamp", func() { GetGenesisTimestamp(Network(99)) })
}

// ---- reward curve ----

// TestBlockRewardCurve pins the documented integer curve:
// reward = 2 + 8*(0.9)^era NG, era = height/1_000_000
func TestBlockRewardCurve(t *testing.T) {
	ng := func(f string) *big.Int {
		v, ok := new(big.Int).SetString(f, 10)
		if !ok {
			t.Fatal("bad constant")
		}
		return v
	}

	cases := []struct {
		height uint64
		want   *big.Int
	}{
		{0, ng("10000000000000000000")},                // 2 + 8
		{rewardEra - 1, ng("10000000000000000000")},    // still era 0
		{rewardEra, ng("9200000000000000000")},         // 2 + 7.2
		{2 * rewardEra, ng("8480000000000000000")},     // 2 + 6.48
		{2*rewardEra + 999, ng("8480000000000000000")}, // era-constant
	}
	for _, c := range cases {
		if got := GetBlockReward(c.height); got.Cmp(c.want) != 0 {
			t.Fatalf("reward@%d = %s, want %s", c.height, got, c.want)
		}
	}

	// the curve decays monotonically towards the 2 NG floor
	prev := GetBlockReward(0)
	for era := uint64(1); era <= 200; era++ {
		got := GetBlockReward(era * rewardEra)
		if got.Cmp(prev) > 0 {
			t.Fatalf("the reward must never rise: era %d", era)
		}
		prev = got
	}
	if prev.Cmp(minReward) < 0 {
		t.Fatal("the reward must never fall below 2 NG")
	}
}

// ---- address ----

func TestAddressBytes(t *testing.T) {
	key, _ := GenerateKey()
	addr := NewAddress(key)

	if !bytes.Equal(addr.Bytes(), addr[:]) {
		t.Fatal("Bytes must expose the raw address")
	}

	rebuilt := Address{}.SetBytes(addr.Bytes())
	if !rebuilt.Equals(addr) {
		t.Fatal("SetBytes must rebuild the address")
	}
	mustPanic(t, "SetBytes(short)", func() { Address{}.SetBytes([]byte{1, 2}) })

	// bs58 round trip
	parsed, err := NewAddressFromBS58(addr.BS58())
	if err != nil || !parsed.Equals(addr) {
		t.Fatalf("bs58 round trip: %v", err)
	}
	if addr.String() != addr.BS58() {
		t.Fatal("String must equal BS58")
	}

	// invalid inputs
	if _, err := NewAddressFromBS58("0OIl"); err == nil {
		t.Fatal("non-base58 digits must be rejected")
	}
	if _, err := NewAddressFromBS58("abc"); err == nil {
		t.Fatal("a wrong-length payload must be rejected")
	}
	mustPanic(t, "mustAddressFromBS58", func() { mustAddressFromBS58("abc") })
}

func TestAddressUnmarshalJSONErrors(t *testing.T) {
	var addr Address
	for i, raw := range []string{`123`, `"0OIl"`, `"abc"`} {
		if err := json.Unmarshal([]byte(raw), &addr); err == nil {
			t.Fatalf("case %d (%s) must fail", i, raw)
		}
	}
}
