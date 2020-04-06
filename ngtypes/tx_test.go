package ngtypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/gogo/protobuf/proto"

	"github.com/ngchain/ngcore/utils"
)

// TestDeserialize test unsigned transaction whether it is possible to deserialize
func TestDeserialize(t *testing.T) {
	tx := NewUnsignedTx(
		TX_GENERATION,
		0,
		[][]byte{GenesisPK},
		[]*big.Int{new(big.Int).Mul(NG, big.NewInt(1000))},
		GetBig0(),
		0,
		nil,
	)

	t.Log(tx.Size())

	raw, _ := proto.Marshal(tx)
	result := hex.EncodeToString(raw)
	t.Log(result)

	var otherTx Tx
	_ = proto.Unmarshal(raw, &otherTx)
	t.Log(otherTx.String())
}

// TestTransaction_Signature test generated Key pair
func TestTransaction_Signature(t *testing.T) {
	o := NewUnsignedTx(0, 1, [][]byte{GenesisPK}, []*big.Int{GetBig0()}, GetBig0(), 0, nil)
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	priv2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	_ = o.Signature(priv)

	if err := o.Verify(priv.PublicKey); err != nil {
		t.Fail()
	}

	if err := o.Verify(priv2.PublicKey); err == nil {
		t.Fail()
	}
}

func TestGetGenesisGenerate(t *testing.T) {
	gg := GetGenesisGeneration()
	if err := gg.Verify(utils.Bytes2ECDSAPublicKey(gg.GetParticipants()[0])); err != nil {
		t.Fail()
	}
}
