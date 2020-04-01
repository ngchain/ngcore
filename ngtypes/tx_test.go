package ngtypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/gogo/protobuf/proto"
)

func TestDeserialize(t *testing.T) {
	tx := NewUnsignedTransaction(
		0,
		0,
		[][]byte{GenesisPK},
		[]*big.Int{new(big.Int).Mul(NG, big.NewInt(1000))},
		Big0,
		0,
		nil,
	)

	t.Log(tx.Size())

	raw, _ := proto.Marshal(tx)
	result := hex.EncodeToString(raw)
	t.Log(result)

	var otherTx Transaction
	_ = proto.Unmarshal(raw, &otherTx)
	t.Log(otherTx.String())
}

func TestTransaction_Signature(t *testing.T) {
	o := NewUnsignedTransaction(0, 1, [][]byte{GenesisPK}, []*big.Int{Big0}, Big0, 0, nil)
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	priv2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	_ = o.Signature(priv)

	if !o.Verify(priv.PublicKey) {
		t.Fail()
	}

	if o.Verify(priv2.PublicKey) {
		t.Fail()
	}
}
