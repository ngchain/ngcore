package ngtypes

import (
	"math/big"
	"testing"
)

// TestSigningHashHeightFlexibility pins the height-flexible reveal property:
// an effect tx (Transact/Deploy) signs the height-INDEPENDENT UnheightedHash,
// so one signature stays valid as the reveal is retargeted across its window;
// a Generate tx signs the height-INCLUSIVE hash, so moving its height breaks
// the signature. MLDSA44 (a full, non-recovery envelope) makes Verify decisive.
func TestSigningHashHeightFlexibility(t *testing.T) {
	key, err := GenerateSchemeKey(SchemeMLDSA44)
	if err != nil {
		t.Fatal(err)
	}
	from := NewAddress(key)

	var to Address
	to[0] = 0xab

	// an effect tx signed once at height 5
	tx := NewTx(ZERONET, TransactTx, 5, to, big.NewInt(1), big.NewInt(1), nil, nil)
	tx.Salt = []byte("mempool-salt-0123456789")
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(tx.SigningHash(), tx.UnheightedHash()) {
		t.Fatal("an effect tx must sign its UnheightedHash")
	}

	// the SAME signature must verify at every height in the window, and From
	// must stay put — the wallet signs once, the node retargets Height freely
	for _, h := range []uint64{5, 6, 7, 8, 42} {
		tx.Height = h
		if err := tx.Verify(nil); err != nil {
			t.Fatalf("effect tx must verify after retarget to height %d: %v", h, err)
		}
		if f, _ := tx.From(); !f.Equals(from) {
			t.Fatalf("From changed after retarget to height %d", h)
		}
	}

	// a Generate signs its height: the reward is height-bound, so the same
	// signature must NOT survive a height change
	gen := NewTx(ZERONET, GenerateTx, 5, from, GetBlockReward(5), big.NewInt(0), nil, nil)
	if err := gen.Signature(key); err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(gen.SigningHash(), gen.GetUnsignedHash()) {
		t.Fatal("a generate must sign its height-inclusive hash")
	}
	if err := gen.Verify(nil); err != nil {
		t.Fatalf("generate must verify at its signed height: %v", err)
	}
	gen.Height = 6
	if err := gen.Verify(nil); err == nil {
		t.Fatal("generate signature must NOT survive a height change")
	}
}

// TestCommitmentSigningHashHeightFlexibility pins the commitment counterpart:
// a commitment signs its height-independent SigningHash, so one signature stays
// valid as the node relays the commitment to a later block. MLDSA44 (a full,
// non-recovery envelope) makes Verify decisive.
func TestCommitmentSigningHashHeightFlexibility(t *testing.T) {
	key, err := GenerateSchemeKey(SchemeMLDSA44)
	if err != nil {
		t.Fatal(err)
	}
	from := NewAddress(key)

	hash := make([]byte, HashSize)
	hash[0] = 0x7c
	c := NewCommitment(ZERONET, 5, hash, big.NewInt(100))
	if err := c.Signature(key); err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(c.SigningHash(), c.SigningHash()) { // stable
		t.Fatal("signing hash must be deterministic")
	}

	// the SAME signature must verify at every height the commitment is relayed to
	for _, h := range []uint64{5, 6, 7, 8, 42} {
		c.Height = h
		if err := c.Verify(nil); err != nil {
			t.Fatalf("commitment must verify after retarget to height %d: %v", h, err)
		}
		if f, _ := c.From(); !f.Equals(from) {
			t.Fatalf("From changed after retarget to height %d", h)
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
