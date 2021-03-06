package ngtypes_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ngchain/secp256k1"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestDeserialize test unsigned transaction whether it is possible to deserialize.
func TestDeserialize(t *testing.T) {
	tx := ngtypes.NewUnsignedTx(
		ngtypes.NetworkType_TESTNET,
		ngtypes.TxType_GENERATE,
		nil,
		0,
		[][]byte{ngtypes.GenesisAddress},
		[]*big.Int{new(big.Int).Mul(ngtypes.NG, big.NewInt(1000))},
		big.NewInt(0),
		nil,
	)

	t.Log(proto.Size(tx))

	raw, _ := utils.Proto.Marshal(tx)
	result := hex.EncodeToString(raw)
	t.Log(result)

	var otherTx ngtypes.Tx
	_ = utils.Proto.Unmarshal(raw, &otherTx)
	t.Log(otherTx.String())
}

// TestTransaction_Signature test generated Key pair.
func TestTransaction_Signature(t *testing.T) {
	o := ngtypes.NewUnsignedTx(
		ngtypes.NetworkType_TESTNET,
		0,
		nil,
		1,
		[][]byte{ngtypes.GenesisAddress},
		[]*big.Int{big.NewInt(0)},
		big.NewInt(0),
		nil,
	)
	priv1, _ := secp256k1.GeneratePrivateKey()
	priv2, _ := secp256k1.GeneratePrivateKey()

	_ = o.Signature(priv1)

	if err := o.Verify(*priv1.PubKey()); err != nil {
		t.Fail()
	}

	if err := o.Verify(*priv2.PubKey()); err == nil {
		t.Fail()
	}
}

func TestGetGenesisGenerate(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		gg := ngtypes.GetGenesisGenerateTx(net)
		if err := gg.Verify(ngtypes.Address(gg.GetParticipants()[0]).PubKey()); err != nil {
			t.Log(err)
			t.Fail()
		}
	}

}

func TestTxJSON(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		tx1 := ngtypes.GetGenesisGenerateTx(net)
		jsonTx, err := utils.JSON.Marshal(tx1)
		if err != nil {
			t.Error(err)
			return
		}

		t.Log(string(jsonTx))

		tx2 := &ngtypes.Tx{}
		err = utils.JSON.Unmarshal(jsonTx, &tx2)
		if err != nil {
			t.Error(err)
			return
		}

		if !proto.Equal(tx1, tx2) {
			t.Error("tx 2 is different from 1")
		}
	}
}
