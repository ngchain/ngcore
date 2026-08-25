package ngtypes

import (
	"math/big"
	"testing"

	"github.com/mr-tron/base58"
)

// signedGenerateTx builds a valid generate tx paying the signer the
// exact block reward at the given height
func signedGenerateTx(t *testing.T, key *PrivateKey, height uint64) *FullTx {
	t.Helper()

	tx := NewUnsignedTx(ZERONET, GenerateTx, height, NewAddress(key),
		GetBlockReward(height), big.NewInt(0), nil)
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}

	return tx
}

func TestCheckGenerate(t *testing.T) {
	key, _ := GenerateKey()

	// the happy path: reward exact, zero fee, paying its own signer
	tx := signedGenerateTx(t, key, 1)
	if err := tx.CheckGenerate(1, nil); err != nil {
		t.Fatalf("valid generate rejected: %v", err)
	}

	// nil receiver
	var nilTx *FullTx
	if err := nilTx.CheckGenerate(1, nil); err == nil {
		t.Fatal("nil generate must be rejected")
	}

	// wrong reward
	bad := NewUnsignedTx(ZERONET, GenerateTx, 1, NewAddress(key),
		big.NewInt(1), big.NewInt(0), nil)
	_ = bad.Signature(key)
	if err := bad.CheckGenerate(1, nil); err == nil {
		t.Fatal("wrong reward must be rejected")
	}

	// nonzero fee (the total still matches the reward)
	reward := GetBlockReward(1)
	value := new(big.Int).Sub(reward, big.NewInt(1))
	bad = NewUnsignedTx(ZERONET, GenerateTx, 1, NewAddress(key), value, big.NewInt(1), nil)
	_ = bad.Signature(key)
	if err := bad.CheckGenerate(1, nil); err == nil {
		t.Fatal("nonzero generate fee must be rejected")
	}

	// unsigned
	bad = NewUnsignedTx(ZERONET, GenerateTx, 1, NewAddress(key), GetBlockReward(1), big.NewInt(0), nil)
	if err := bad.CheckGenerate(1, nil); err == nil {
		t.Fatal("unsigned generate must be rejected")
	}

	// paying someone else than the signer
	other, _ := GenerateKey()
	bad = NewUnsignedTx(ZERONET, GenerateTx, 1, NewAddress(other), GetBlockReward(1), big.NewInt(0), nil)
	_ = bad.Signature(key)
	if err := bad.CheckGenerate(1, nil); err == nil {
		t.Fatal("generate paying a foreign address must be rejected")
	}
}

func TestCheckNoTransferTypes(t *testing.T) {
	key, _ := GenerateKey()

	type check func(*FullTx) error
	cases := map[string]struct {
		txType TxType
		run    check
	}{
		"deploy": {DeployTx, func(tx *FullTx) error { return tx.CheckDeploy(nil) }},
	}

	for name, c := range cases {
		// valid: no To, no value, signed
		tx := NewUnsignedTx(ZERONET, c.txType, 1, Address{}, big.NewInt(0), big.NewInt(1), []byte{1})
		if err := tx.Signature(key); err != nil {
			t.Fatal(err)
		}
		if err := c.run(tx); err != nil {
			t.Fatalf("%s: valid tx rejected: %v", name, err)
		}

		// a To address set
		tx = NewUnsignedTx(ZERONET, c.txType, 1, NewAddress(key), big.NewInt(0), big.NewInt(0), nil)
		_ = tx.Signature(key)
		if err := c.run(tx); err == nil {
			t.Fatalf("%s: tx with To must be rejected", name)
		}

		// a value set
		tx = NewUnsignedTx(ZERONET, c.txType, 1, Address{}, big.NewInt(1), big.NewInt(0), nil)
		_ = tx.Signature(key)
		if err := c.run(tx); err == nil {
			t.Fatalf("%s: tx with value must be rejected", name)
		}
	}

	// nil receivers
	var nilTx *FullTx
	if nilTx.CheckDeploy(nil) == nil ||
		nilTx.CheckTransaction(nil) == nil {
		t.Fatal("nil txs must be rejected")
	}
}

func TestCheckTransaction(t *testing.T) {
	key, _ := GenerateKey()

	tx := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(10), big.NewInt(1), nil)
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if err := tx.CheckTransaction(nil); err != nil {
		t.Fatalf("valid transaction rejected: %v", err)
	}

	// the negative-value check fires before signature verification
	// (signing such a tx is impossible: rlp refuses negative big.Ints)
	bad := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(-1), big.NewInt(0), nil)
	if err := bad.CheckTransaction(nil); err == nil {
		t.Fatal("negative value must be rejected")
	}
}

func TestTxAccessors(t *testing.T) {
	key, _ := GenerateKey()
	tx := testTx(NewAddress(key))

	if tx.IsSigned() {
		t.Fatal("unsigned tx must not report signed")
	}
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if !tx.IsSigned() {
		t.Fatal("signed tx must report signed")
	}

	if got := tx.TotalExpenditure(); got.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("total expenditure = %s, want 1", got)
	}

	if tx.ID() == "" {
		t.Fatal("tx id must not be empty")
	}
	if _, err := base58.FastBase58Decoding(tx.BS58()); err != nil {
		t.Fatalf("tx BS58 must decode: %v", err)
	}

	// NewTx nil-fills value/fee
	filled := NewTx(ZERONET, TransactTx, 1, Address{}, nil, nil, nil, nil)
	if filled.Value == nil || filled.Fee == nil || filled.Extra == nil || filled.Sign == nil {
		t.Fatal("NewTx must fill nil fields")
	}
}

func TestVerifyExtraExcess(t *testing.T) {
	key, _ := GenerateKey()
	tx := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(0), big.NewInt(0), nil)
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}

	tx.Extra = make([]byte, TxMaxExtraSize+1)
	if err := tx.Verify(nil); err == nil {
		t.Fatal("oversized extra must be rejected")
	}

	// a genesis-height tx skips every check
	tx.Height = 0
	if err := tx.Verify(nil); err != nil {
		t.Fatalf("height-0 tx must pass: %v", err)
	}
}

func TestTxEquals(t *testing.T) {
	key, _ := GenerateKey()
	base := func() *FullTx {
		return NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(2), big.NewInt(1), []byte{9})
	}

	a := base()
	if eq, _ := a.Equals(base()); !eq {
		t.Fatal("identical txs must be equal")
	}

	cases := []*FullTx{base(), base(), base(), base(), base(), base()}
	cases[0].Network = TESTNET
	cases[1].Height = 2
	cases[2].To = Address{1}
	cases[3].Value = big.NewInt(3)
	cases[4].Fee = big.NewInt(4)
	cases[5].Extra = []byte{8}

	for i, c := range cases {
		if eq, _ := a.Equals(c); eq {
			t.Fatalf("case %d: differing txs must not be equal", i)
		}
	}

	// comparing against a non-tx panics by contract
	defer func() {
		if recover() == nil {
			t.Fatal("comparing a tx to a non-tx must panic")
		}
	}()
	_, _ = a.Equals(&BlockHeader{})
}
