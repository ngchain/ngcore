package ngtypes

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
)

// buildSealedBlock assembles a height-1 ZERONET block over the given
// txs (a valid generate first) and seals it with a zero nonce; ZERONET
// diff 1 accepts any nonce
func buildSealedBlock(t *testing.T, txs []*FullTx) *FullBlock {
	t.Helper()

	block := NewBareBlock(ZERONET, 1, uint64(time.Now().Unix()),
		make([]byte, HashSize), big.NewInt(1))
	if err := block.ToUnsealing(txs); err != nil {
		t.Fatal(err)
	}
	if err := block.ToSealed(make([]byte, NonceSize)); err != nil {
		t.Fatal(err)
	}

	return block
}

func TestBlockLifecycle(t *testing.T) {
	key, _ := GenerateKey()
	gtx := signedGenerateTx(t, key, 1)
	block := buildSealedBlock(t, []*FullTx{gtx})

	if err := block.CheckError(); err != nil {
		t.Fatalf("a fully valid block must check clean: %v", err)
	}

	if !block.IsUnsealing() || !block.IsSealed() {
		t.Fatal("the sealed block must report unsealing+sealed")
	}
	if block.IsGenesis() {
		t.Fatal("a height-1 block is not the genesis")
	}
	if block.GetActualDiff().Sign() <= 0 {
		t.Fatal("the actual diff must be positive")
	}

	// header accessors
	if block.GetHeight() != 1 {
		t.Fatal("wrong height")
	}
	if !bytes.Equal(block.GetPrevHash(), make([]byte, HashSize)) {
		t.Fatal("wrong prev hash")
	}
	if hash, err := block.BlockHeader.CalculateHash(); err != nil || !bytes.Equal(hash, block.GetHash()) {
		t.Fatal("the header hash must match the block hash")
	}
	if !bytes.Equal(block.BlockHeader.GetHash(), block.GetHash()) {
		t.Fatal("the header GetHash must match the block hash")
	}

	// tx accessors
	if block.GetTx(0) == nil || block.GetTx(1) != nil || block.GetTx(-1) != nil {
		t.Fatal("GetTx bounds are wrong")
	}

	// the PoW raw header takes an override nonce
	withNonce := block.GetPoWRawHeader([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	if bytes.Equal(withNonce, block.GetPoWRawHeader(nil)) {
		t.Fatal("the override nonce must change the raw header")
	}
}

func TestBlockHeadTail(t *testing.T) {
	mk := func(height uint64) *FullBlock {
		return NewBareBlock(ZERONET, height, 0, make([]byte, HashSize), big.NewInt(1))
	}

	if !mk(BlockCheckRound).IsHead() || mk(BlockCheckRound+1).IsHead() {
		t.Fatal("IsHead is wrong")
	}
	if !mk(BlockCheckRound-1).IsTail() || mk(BlockCheckRound).IsTail() {
		t.Fatal("IsTail is wrong")
	}
}

func TestToUnsealingRejects(t *testing.T) {
	key, _ := GenerateKey()
	gtx := signedGenerateTx(t, key, 1)

	bare := func() *FullBlock {
		return NewBareBlock(ZERONET, 1, uint64(time.Now().Unix()), make([]byte, HashSize), big.NewInt(1))
	}

	// the first tx must be a generate
	transact := testTx(NewAddress(key))
	if err := bare().ToUnsealing([]*FullTx{transact}); !errors.Is(err, ErrBlockNoGen) {
		t.Fatalf("want ErrBlockNoGen, got %v", err)
	}

	// only one generate allowed
	if err := bare().ToUnsealing([]*FullTx{gtx, signedGenerateTx(t, key, 1)}); !errors.Is(err, ErrBlockOnlyOneGen) {
		t.Fatalf("want ErrBlockOnlyOneGen, got %v", err)
	}

	// every follow-up tx locks to the block height
	wrongHeight := NewUnsignedTx(ZERONET, TransactTx, 9, NewAddress(key), big.NewInt(1), big.NewInt(0), nil)
	_ = wrongHeight.Signature(key)
	if err := bare().ToUnsealing([]*FullTx{gtx, wrongHeight}); !errors.Is(err, ErrTxExtraInvalid) {
		t.Fatalf("want ErrTxExtraInvalid, got %v", err)
	}
}

func TestToSealedRejects(t *testing.T) {
	// a bare block (no tx trie yet) cannot seal
	bare := &FullBlock{BlockHeader: &BlockHeader{Network: ZERONET, Height: 1}}
	if err := bare.ToSealed(make([]byte, NonceSize)); !errors.Is(err, ErrBlockSealBare) {
		t.Fatalf("want ErrBlockSealBare, got %v", err)
	}

	key, _ := GenerateKey()
	block := NewBareBlock(ZERONET, 1, uint64(time.Now().Unix()), make([]byte, HashSize), big.NewInt(1))
	if err := block.ToUnsealing([]*FullTx{signedGenerateTx(t, key, 1)}); err != nil {
		t.Fatal(err)
	}
	if err := block.ToSealed([]byte{1, 2, 3}); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("want ErrInvalidNonce, got %v", err)
	}
}

func TestCheckErrorRejects(t *testing.T) {
	key, _ := GenerateKey()
	gtx := signedGenerateTx(t, key, 1)

	fresh := func() *FullBlock { return buildSealedBlock(t, []*FullTx{gtx}) }

	// too many txs
	block := fresh()
	block.Txs = make([]*FullTx, MaxBlockTxCount+1)
	for i := range block.Txs {
		block.Txs[i] = gtx
	}
	if err := block.CheckError(); !errors.Is(err, ErrBlockTxsExcess) {
		t.Fatalf("want ErrBlockTxsExcess, got %v", err)
	}

	// too many bytes
	block = fresh()
	fat := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(0), big.NewInt(0),
		make([]byte, MaxBlockBytes))
	block.Txs = []*FullTx{gtx, fat}
	if err := block.CheckError(); !errors.Is(err, ErrBlockBytesExcess) {
		t.Fatalf("want ErrBlockBytesExcess, got %v", err)
	}

	// malformed header fields
	block = fresh()
	block.BlockHeader.PrevBlockHash = []byte{1}
	if err := block.CheckError(); !errors.Is(err, ErrBlockPrevHashInvalid) {
		t.Fatalf("want ErrBlockPrevHashInvalid, got %v", err)
	}

	block = fresh()
	block.BlockHeader.TxTrieHash = []byte{1}
	if err := block.CheckError(); !errors.Is(err, ErrBlockTxTrieHashInvalid) {
		t.Fatalf("want ErrBlockTxTrieHashInvalid, got %v", err)
	}

	block = fresh()
	block.BlockHeader.Nonce = []byte{1}
	if err := block.CheckError(); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("want ErrInvalidNonce, got %v", err)
	}

	// a far-future timestamp
	block = fresh()
	block.BlockHeader.Timestamp = uint64(time.Now().UnixMilli()) + TimestampDriftTolerance + 100
	if err := block.CheckError(); !errors.Is(err, ErrBlockTimestampInvalid) {
		t.Fatalf("want ErrBlockTimestampInvalid, got %v", err)
	}

	// a wrong tx trie commitment
	block = fresh()
	block.BlockHeader.TxTrieHash = make([]byte, HashSize)
	if err := block.CheckError(); !errors.Is(err, ErrBlockTxTrieHashInvalid) {
		t.Fatalf("want trie mismatch, got %v", err)
	}

	// a wrong witness commitment
	block = fresh()
	block.BlockHeader.WitnessRoot = make([]byte, HashSize)
	if err := block.CheckError(); !errors.Is(err, ErrBlockWitnessRootInvalid) {
		t.Fatalf("want ErrBlockWitnessRootInvalid, got %v", err)
	}

	// an impossible difficulty: target 1, so any pow hash fails
	block = fresh()
	block.BlockHeader.Difficulty = MaxTarget.Bytes()
	if err := block.CheckError(); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("want nonce rejection under max diff, got %v", err)
	}
}

func TestBlockEquals(t *testing.T) {
	key, _ := GenerateKey()
	gtx := signedGenerateTx(t, key, 1)

	a := buildSealedBlock(t, []*FullTx{gtx})
	b := buildSealedBlock(t, []*FullTx{gtx})
	if eq, _ := a.Equals(b); !eq {
		t.Fatal("identical blocks must be equal")
	}

	// a different header
	c := buildSealedBlock(t, []*FullTx{gtx})
	c.BlockHeader.Height = 2
	if eq, _ := a.Equals(c); eq {
		t.Fatal("differing headers must not be equal")
	}

	// a different tx count
	c = buildSealedBlock(t, []*FullTx{gtx})
	c.Txs = append(c.Txs, gtx)
	if eq, _ := a.Equals(c); eq {
		t.Fatal("differing tx counts must not be equal")
	}

	// a different tx
	other := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(7), big.NewInt(0), nil)
	_ = other.Signature(key)
	c = buildSealedBlock(t, []*FullTx{gtx})
	c.Txs = []*FullTx{other}
	if eq, _ := a.Equals(c); eq {
		t.Fatal("differing txs must not be equal")
	}
}

func TestBlockHeaderEquals(t *testing.T) {
	base := func() *BlockHeader {
		return &BlockHeader{
			Network:       ZERONET,
			Height:        1,
			Timestamp:     2,
			PrevBlockHash: []byte{1},
			TxTrieHash:    []byte{2},
			WitnessRoot:   []byte{3},
			Difficulty:    []byte{4},
			Nonce:         []byte{5},
		}
	}

	a := base()
	if eq, err := a.Equals(base()); err != nil || !eq {
		t.Fatal("identical headers must be equal")
	}

	// a non-header content errors
	if _, err := a.Equals(testTx(Address{})); !errors.Is(err, ErrNotBlockHeader) {
		t.Fatalf("want ErrNotBlockHeader, got %v", err)
	}

	muts := []func(*BlockHeader){
		func(h *BlockHeader) { h.Network = TESTNET },
		func(h *BlockHeader) { h.Height = 9 },
		func(h *BlockHeader) { h.Timestamp = 9 },
		func(h *BlockHeader) { h.PrevBlockHash = []byte{9} },
		func(h *BlockHeader) { h.TxTrieHash = []byte{9} },
		func(h *BlockHeader) { h.WitnessRoot = []byte{9} },
		func(h *BlockHeader) { h.Difficulty = []byte{9} },
		func(h *BlockHeader) { h.Nonce = []byte{9} },
	}
	for i, mut := range muts {
		h := base()
		mut(h)
		if eq, _ := a.Equals(h); eq {
			t.Fatalf("mutation %d must break equality", i)
		}
	}
}
