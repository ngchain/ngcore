package ngpool

import (
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestGetCommitPackDedupsByHash proves a hash-copying attacker cannot make the
// miner assemble an invalid block: the block layer rejects two commitments with
// the same hash, so a copycat that reuses a victim's PUBLIC blind hash under a
// different From must be de-duplicated at pack time. Two same-hash commitments
// pack down to ONE — the higher-fee one — so the assembled block is always valid.
func TestGetCommitPackDedupsByHash(t *testing.T) {
	var a, b, c ngtypes.Address
	a[0], b[0], c[0] = 0xa, 0xb, 0xc

	h := make([]byte, ngtypes.HashSize)
	h[0] = 0x11
	other := make([]byte, ngtypes.HashSize)
	other[0] = 0x22

	// same blind hash h, different committers; a outbids b
	ca := ngtypes.NewCommitment(ngtypes.ZERONET, 5, h, big.NewInt(200))
	cb := ngtypes.NewCommitment(ngtypes.ZERONET, 5, h, big.NewInt(100))
	cc := ngtypes.NewCommitment(ngtypes.ZERONET, 5, other, big.NewInt(100))

	pool := &TxPool{commitMap: map[ngtypes.Address]*ngtypes.Commitment{a: ca, b: cb, c: cc}}
	packed := pool.GetCommitPack(5)

	seen := map[string]int{}
	for _, p := range packed {
		seen[string(p.Hash)]++
	}
	if seen[string(h)] != 1 {
		t.Fatalf("the duplicated hash was packed %d times, want 1 (a duplicate makes the block invalid)", seen[string(h)])
	}
	if seen[string(other)] != 1 {
		t.Fatal("the distinct-hash commitment must still be packed")
	}
	if len(packed) != 2 {
		t.Fatalf("packed %d commitments, want 2", len(packed))
	}
	for _, p := range packed {
		if string(p.Hash) == string(h) && p.Fee.Cmp(big.NewInt(200)) != 0 {
			t.Fatal("dedup kept the lower-fee duplicate, want the higher-fee one")
		}
	}
}

// TestGetPackDedupsByTxid is the tx counterpart: two reveals with identical
// content (hence the same txid, which excludes the signature) but different
// signers pack down to one, so a content-copying reveal cannot force an invalid
// (duplicate-txid) block either.
func TestGetPackDedupsByTxid(t *testing.T) {
	var to ngtypes.Address
	to[0] = 0x42

	// identical content -> identical txid; only the (unsigned) content matters
	mk := func(fee int64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 5, to,
			big.NewInt(1), big.NewInt(fee), nil, nil)
	}
	t1, t2 := mk(1), mk(1)

	var a, b ngtypes.Address
	a[0], b[0] = 0x1, 0x2
	pool := &TxPool{txMap: map[ngtypes.Address]*ngtypes.FullTx{a: t1, b: t2}}
	trie := pool.GetPack(5)

	if n := len(trie); n != 1 {
		t.Fatalf("packed %d txs with the same txid, want 1 (a duplicate makes the block invalid)", n)
	}
}
