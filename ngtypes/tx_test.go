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
		ngtypes.TxType_GENERATE,
		0,
		[][]byte{ngtypes.GenesisPublicKey},
		[]*big.Int{new(big.Int).Mul(ngtypes.NG, big.NewInt(1000))},
		ngtypes.GetBig0(),
		0,
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
		0,
		1,
		[][]byte{ngtypes.GenesisPublicKey},
		[]*big.Int{ngtypes.GetBig0()},
		ngtypes.GetBig0(),
		0,
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
	gg := ngtypes.GetGenesisGenerateTx()
	if err := gg.Verify(utils.Bytes2PublicKey(gg.Header.GetParticipants()[0])); err != nil {
		t.Log(err)
		t.Fail()
	}
}
