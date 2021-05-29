package ngtypes_test

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/ngtypes/ngproto"
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

		raw, err := proto.Marshal(g.GetProto())
		if err != nil {
			panic(err)
		}
		gg := new(ngproto.Block)
		err = proto.Unmarshal(raw, gg)
		if err != nil {
			panic(err)
		}

		ggg := ngtypes.NewBlockFromProto(gg)

		if !ggg.IsGenesis() {
			t.Error("failed unmarshalling back to genesis block structure")
			return
		}

		if err := ggg.CheckError(); err != nil {
			t.Error(err)
			return
		}
	}

}

// TestBlock_Marshal test func GetGenesisBlock()'s Marshal().
func TestBlock_Marshal(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		rawBlock, _ := ngtypes.GetGenesisBlock(net).Marshal()

		var genesisBlock ngproto.Block
		_ = proto.Unmarshal(rawBlock, &genesisBlock)
		_block, _ := proto.Marshal(&genesisBlock)

		if !bytes.Equal(rawBlock, _block) {
			t.Fail()
		}
	}
}

// TestGetGenesisBlock test func GetGenesisBlock()'s parameter passing.
func TestGetGenesisBlock(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		t.Logf(string(net))
		d, _ := proto.Marshal(ngtypes.GetGenesisBlock(net).GetProto())
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

		block_ := &ngtypes.Block{}
		err = utils.JSON.Unmarshal(jsonBlock, &block_)
		if err != nil {
			t.Error(err)
			return
		}

		if eq, _ := block.Equals(block_); !eq {
			log.Errorf("block  %#v", block)
			log.Errorf("block_ %#v", block_)
			t.Fail()
		}

		if !proto.Equal(block.GetProto(), block_.GetProto()) {
			log.Errorf("block  %#v", block)
			log.Errorf("block_ %#v", block_)
			t.Fail()
		}
	}
}

func TestBlockRawPoW(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		block := ngtypes.GetGenesisBlock(net)
		raw := block.GetPoWRawHeader(nil)
		txs := block.Txs
		block_, err := ngtypes.NewBlockFromPoWRawWithTxs(raw, txs)
		if err != nil {
			panic(err)
		}

		if eq, _ := block.Equals(block_); !eq {
			log.Errorf("block  %#v", block)
			log.Errorf("block_ %#v", block_)
			t.Fail()
		}

		if !proto.Equal(block.GetProto(), block_.GetProto()) {
			log.Errorf("block  %#v", block)
			log.Errorf("block_ %#v", block_)
			t.Fail()
		}
	}
}
