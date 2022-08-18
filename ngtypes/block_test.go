package ngtypes_test

import (
	"bytes"
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
		t.Logf(string(net))
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

func TestBlockRawPoW(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		block := ngtypes.GetGenesisBlock(net)
		raw := block.GetPoWRawHeader(nil)
		txs := block.Txs
		block2, err := ngtypes.NewBlockFromPoWRaw(raw, txs, nil)
		if err != nil {
			panic(err)
		}

		if eq, _ := block.Equals(block2); !eq {
			log.Errorf("block  %#v", block)
			log.Errorf("block2 %#v", block2)
			t.Fail()
		}
	}
}
