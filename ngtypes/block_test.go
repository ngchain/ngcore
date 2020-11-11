package ngtypes_test

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"

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

		raw, _ := utils.Proto.Marshal(g)
		gg := new(ngtypes.Block)
		_ = utils.Proto.Unmarshal(raw, gg)

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
		block, _ := utils.Proto.Marshal(ngtypes.GetGenesisBlock(net))

		var genesisBlock ngtypes.Block
		_ = utils.Proto.Unmarshal(block, &genesisBlock)
		_block, _ := utils.Proto.Marshal(&genesisBlock)

		if !bytes.Equal(block, _block) {
			t.Fail()
		}
	}
}

// TestGetGenesisBlock test func GetGenesisBlock()'s parameter passing.
func TestGetGenesisBlock(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		t.Logf(string(net))
		d, _ := utils.Proto.Marshal(ngtypes.GetGenesisBlock(net))
		hash := sha3.Sum256(d)

		t.Logf("GenesisBlock hex: %x", d)
		t.Logf("GenesisBlock hash: %x", hash)
		t.Logf("GenesisBlock Size: %d bytes", len(d))
	}
}

func TestBlockJSON(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		block1 := ngtypes.GetGenesisBlock(net)
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
}

func TestBlockRawPoW(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		block := ngtypes.GetGenesisBlock(net)
		raw := block.GetPoWRawHeader(nil)
		txs := block.Txs
		block_ := new(ngtypes.Block)
		err := block_.ApplyPoWRawAndTxs(raw, txs)
		if err != nil {
			panic(err)
		}
		if !proto.Equal(block, block_) {
			fmt.Println("block", block)
			fmt.Println("block_", block_)
			t.Fail()
		}
	}
}
