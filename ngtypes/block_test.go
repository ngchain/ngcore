package ngtypes_test

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestBlock_GetHash test func GetGenesisBlock() and return Hash value.
func TestBlock_GetHash(t *testing.T) {
	b := ngtypes.GetGenesisBlock()
	headerHash := b.CalculateHeaderHash()
	t.Log(len(headerHash))
}

func TestBlock_IsGenesis(t *testing.T) {
	g := ngtypes.GetGenesisBlock()
	if !g.IsGenesis() {
		t.Fail()
	}

	if err := g.CheckError(); err != nil {
		t.Log(err)
		t.Fail()
	}

	raw, _ := utils.Proto.Marshal(g)
	gg := new(ngtypes.Block)
	_ = utils.Proto.Unmarshal(raw, gg)

	if !gg.IsGenesis() {
		t.Fail()
	}

	if err := gg.CheckError(); err != nil {
		t.Log(err)
		t.Fail()
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

	log.Infof("GenesisBlock hex: %x", d)
	log.Infof("GenesisBlock hash: %x", hash)
}
