package ngtypes_test

import (
	"encoding/hex"
	"github.com/c0mm4nd/rlp"
	"math/big"
	"reflect"
	"testing"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestDeserialize test unsigned transaction whether it is possible to deserialize.
func TestDeserialize(t *testing.T) {
	tx := ngtypes.NewUnsignedTx(
		ngtypes.TESTNET,
		ngtypes.GenerateTx,
		0,
		0,
		[]ngtypes.Address{ngtypes.GenesisAddress},
		[]*big.Int{new(big.Int).Mul(ngtypes.NG, big.NewInt(1000))},
		big.NewInt(0),
		nil,
	)

	raw, _ := rlp.EncodeToBytes(tx)
	t.Log(len(raw))
	result := hex.EncodeToString(raw)
	t.Log(result)

	var otherTx ngtypes.Tx
	_ = rlp.DecodeBytes(raw, &otherTx)
	t.Logf("%#v", otherTx)
}

// TestTransaction_Signature test generated Key pair.
func TestTransaction_Signature(t *testing.T) {
	o := ngtypes.NewUnsignedTx(
		ngtypes.TESTNET,
		0,
		0,
		1,
		[]ngtypes.Address{ngtypes.GenesisAddress},
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
		if err := gg.Verify(gg.Participants[0].PubKey()); err != nil {
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

		if eq, _ := tx1.Equals(tx2); !eq {
			t.Errorf("tx \n 2 %#v \n is different from \n 1 %#v", tx2, tx1)
		}

		if !reflect.DeepEqual(tx1, tx2) {
			t.Errorf("tx \n 2 %#v \n is different from \n 1 %#v", tx2, tx1)
		}
	}
}
