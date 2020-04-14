package ngtypes

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ngchain/secp256k1"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/utils"
)

// TestDeserialize test unsigned transaction whether it is possible to deserialize
func TestDeserialize(t *testing.T) {
	tx := NewUnsignedTx(
		TxType_GENERATE,
		0,
		[][]byte{GenesisPublicKey},
		[]*big.Int{new(big.Int).Mul(NG, big.NewInt(1000))},
		GetBig0(),
		0,
		nil,
	)

	t.Log(proto.Size(tx))

	raw, _ := utils.Proto.Marshal(tx)
	result := hex.EncodeToString(raw)
	t.Log(result)

	var otherTx Tx
	_ = proto.Unmarshal(raw, &otherTx)
	t.Log(otherTx.String())
}

// TestTransaction_Signature test generated Key pair
func TestTransaction_Signature(t *testing.T) {
	o := NewUnsignedTx(0, 1, [][]byte{GenesisPublicKey}, []*big.Int{GetBig0()}, GetBig0(), 0, nil)
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
	gg := GetGenesisGenerateTx()
	if err := gg.Verify(utils.Bytes2PublicKey(gg.GetParticipants()[0])); err != nil {
		t.Log(err)
		t.Fail()
	}
}
