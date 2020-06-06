package ngtypes_test

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestBlock_GetHash test func GetGenesisBlock() and return Hash value.
func TestBlock_GetHash(t *testing.T) {
	b := ngtypes.GetGenesisBlock()
	headerHash := b.CalculateHeaderHash()
	if len(headerHash) != 32 {
		t.Errorf("bytes from CalculateHeaderHash is not hash")
	}
}

func TestBlock_IsGenesis(t *testing.T) {
	g := ngtypes.GetGenesisBlock()
	if !g.IsGenesis() {
		t.Fail()
	}

	if err := g.CheckError(); err != nil {
		t.Error(err)
		return
	}

	raw, _ := utils.Proto.Marshal(g)
	gg := new(ngtypes.Block)
	_ = utils.Proto.Unmarshal(raw, gg)

	if !gg.IsGenesis() {
		t.Error("failed unmarshaling back to genesis block structure")
		return
	}

	if err := gg.CheckError(); err != nil {
		t.Error(err)
		return
	}
}

// TestBlock_Marshal test func GetGenesisBlock()'s Marshal().
func TestBlock_Marshal(t *testing.T) {
	block, _ := utils.Proto.Marshal(ngtypes.GetGenesisBlock())

	var genesisBlock ngtypes.Block
	_ = utils.Proto.Unmarshal(block, &genesisBlock)
	_block, _ := utils.Proto.Marshal(&genesisBlock)

	if !bytes.Equal(block, _block) {
		t.Fail()
	}
}

// TestGetGenesisBlock test func GetGenesisBlock()'s parameter passing.
func TestGetGenesisBlock(t *testing.T) {
	d, _ := utils.Proto.Marshal(ngtypes.GetGenesisBlock())
	hash := sha3.Sum256(d)

	t.Logf("GenesisBlock hex: %x", d)
	t.Logf("GenesisBlock hash: %x", hash)
	t.Logf("GenesisBlock Size: %d bytes", len(d))
}

func TestBlockJSON(t *testing.T) {
	block1 := ngtypes.GetGenesisBlock()
	jsonBlock, err := utils.JSON.Marshal(block1)
	if err != nil {
		t.Error(err)
		return
	}

	t.Log(string(jsonBlock))

	block2 := &ngtypes.Block{}
	err = utils.JSON.Unmarshal(jsonBlock, &block2)
	if err != nil {
		t.Error(err)
		return
	}

	if !proto.Equal(block1, block2) {
		t.Error("block 2 is different from 1")
	}
}
