package ngtypes_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/c0mm4nd/rlp"
	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func TestPowHash(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		b := ngtypes.GetGenesisBlock(net)
		headerHash := b.PowHash()
		if len(headerHash) != ngtypes.HashSize {
			t.Errorf("pow hash %x is not a valid hash", headerHash)
		}
	}
}

func TestBlock_IsGenesis(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		t.Log(net)

		g := ngtypes.GetGenesisBlock(net)
		if !g.IsGenesis() {
			t.Fail()
		}

		if err := g.CheckError(); err != nil {
			t.Error(err)
			return
		}

		raw, err := rlp.EncodeToBytes(g)
		if err != nil {
			panic(err)
		}
		gg := new(ngtypes.FullBlock)
		err = rlp.DecodeBytes(raw, gg)
		if err != nil {
			panic(err)
		}

		if !gg.IsGenesis() {
			t.Error("failed unmarshalling back to genesis block structure")
			return
		}

		if err := gg.CheckError(); err != nil {
			t.Error(err)
			return
		}
	}
}

// TestBlock_Marshal test func GetGenesisBlock()'s Marshal().
func TestBlock_Marshal(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		rawBlock, _ := rlp.EncodeToBytes(ngtypes.GetGenesisBlock(net))

		var genesisBlock ngtypes.FullBlock
		_ = rlp.DecodeBytes(rawBlock, &genesisBlock)
		_block, _ := rlp.EncodeToBytes(&genesisBlock)

		if !bytes.Equal(rawBlock, _block) {
			t.Fail()
		}
	}
}

// TestGetGenesisBlock test func GetGenesisBlock()'s parameter passing.
func TestGetGenesisBlock(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		t.Logf("%s", string(net))
		d, _ := rlp.EncodeToBytes(ngtypes.GetGenesisBlock(net))
		hash := sha3.Sum256(d)

		t.Logf("GenesisBlock hex: %x", d)
		t.Logf("GenesisBlock hash: %x", hash)
		t.Logf("GenesisBlock Size: %d bytes", len(d))
	}
}

func TestBlockJSON(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		block := ngtypes.GetGenesisBlock(net)
		jsonBlock, err := utils.JSON.Marshal(block)
		if err != nil {
			t.Error(err)
			return
		}

		t.Log(string(jsonBlock))

		block2 := &ngtypes.FullBlock{}
		err = utils.JSON.Unmarshal(jsonBlock, &block2)
		if err != nil {
			t.Error(err)
			return
		}

		if eq, _ := block.Equals(block2); !eq {
			log.Errorf("block  %#v", block)
			log.Errorf("block2 %#v", block2)
			t.Fail()
		}

		if eq, _ := block.Equals(block2); !eq {
			log.Errorf("block  %#v", block)
			log.Errorf("block2 %#v", block2)
			t.Fail()
		}
	}
}

// TestWitnessSeparation pins the segwit-style split: the txid ignores
// the signature envelope, the witness root commits it, and a block
// whose witness bytes were swapped is rejected even though every txid
// (and so the tx trie root) stays identical
func TestWitnessSeparation(t *testing.T) {
	// a non-recovery scheme: its full/compact envelopes differ and its
	// hedged signing yields fresh bytes per signature, which is what
	// this test needs to vary the witness while txids stay put
	key, _ := ngtypes.GenerateSchemeKey(ngtypes.SchemeFNDSA512)

	tx := ngtypes.NewUnsignedTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		ngtypes.NewAddress(key), big.NewInt(1), big.NewInt(0), nil)
	unsignedID := tx.GetHash()

	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tx.GetHash(), unsignedID) {
		t.Fatal("the txid must not change when the tx gets signed")
	}

	// same tx, compact envelope: same txid, different witness root
	compact := ngtypes.NewUnsignedTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		ngtypes.NewAddress(key), big.NewInt(1), big.NewInt(0), nil)
	if err := compact.SignatureCompact(key); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact.GetHash(), tx.GetHash()) {
		t.Fatal("envelope form must not affect the txid")
	}
	full := ngtypes.CalcWitnessRoot([]*ngtypes.FullTx{tx})
	comp := ngtypes.CalcWitnessRoot([]*ngtypes.FullTx{compact})
	if bytes.Equal(full, comp) {
		t.Fatal("different witness bytes must yield different witness roots")
	}

	// a sealed block must reject swapped witness bytes: the tx trie
	// root still matches, the witness root does not
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	height := uint64(1)
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + 16

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(key), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(key); err != nil {
		t.Fatal(err)
	}

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, genesis.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, genesis))
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}

	sealed := false
	for n := uint64(0); n < 1_000_000; n++ {
		nonce := make([]byte, ngtypes.NonceSize)
		binary.LittleEndian.PutUint64(nonce, n)
		if err := block.ToSealed(nonce); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			sealed = true
			break
		}
	}
	if !sealed {
		t.Fatal("failed to seal the test block")
	}

	// re-sign the SAME generate tx: txid identical, witness differs
	resigned := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(key), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := resigned.SignatureCompact(key); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(resigned.GetHash(), genTx.GetHash()) {
		t.Fatal("re-signed tx must keep its txid")
	}

	block.Txs = []*ngtypes.FullTx{resigned}
	if err := block.CheckError(); err == nil {
		t.Fatal("swapped witness bytes must invalidate the block")
	} else if !errors.Is(err, ngtypes.ErrBlockWitnessRootInvalid) {
		t.Fatalf("got %v, want ErrBlockWitnessRootInvalid", err)
	}
}

// TestBlockCapacity: the tx-count and byte-size caps are CONSENSUS
// rules — an overstuffed block fails CheckError outright
func TestBlockCapacity(t *testing.T) {
	key, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	height := uint64(1)
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + 16

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(key), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(key); err != nil {
		t.Fatal(err)
	}

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, genesis.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, genesis))
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	sealed := false
	for n := uint64(0); n < 1_000_000; n++ {
		nonce := make([]byte, ngtypes.NonceSize)
		binary.LittleEndian.PutUint64(nonce, n)
		if err := block.ToSealed(nonce); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			sealed = true
			break
		}
	}
	if !sealed {
		t.Fatal("failed to seal")
	}

	// stuff the body past the tx-count cap: the capacity check fires
	// before anything else
	overstuffed := make([]*ngtypes.FullTx, 0, ngtypes.MaxBlockTxCount+1)
	for i := 0; i <= ngtypes.MaxBlockTxCount; i++ {
		overstuffed = append(overstuffed, genTx)
	}
	block.Txs = overstuffed
	if err := block.CheckError(); !errors.Is(err, ngtypes.ErrBlockTxsExcess) {
		t.Fatalf("got %v, want ErrBlockTxsExcess", err)
	}

	// a few megabyte-extra txs blow the byte cap
	bigExtra := make([]byte, 1<<20)
	big1 := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
		ngtypes.NewAddress(key), big.NewInt(0), big.NewInt(0), bigExtra, nil)
	if err := big1.Signature(key); err != nil {
		t.Fatal(err)
	}
	fat := make([]*ngtypes.FullTx, 0, 9)
	for i := 0; i < 9; i++ {
		fat = append(fat, big1)
	}
	block.Txs = fat
	if err := block.CheckError(); !errors.Is(err, ngtypes.ErrBlockBytesExcess) {
		t.Fatalf("got %v, want ErrBlockBytesExcess", err)
	}
}
