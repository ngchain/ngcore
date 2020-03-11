package ngtypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math"
	"testing"

	"github.com/NebulousLabs/fastrand"
)

func TestNewAccount(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Error(err)
	}

	randUint64 := fastrand.Uint64n(math.MaxUint64)
	acc := NewAccount(
		randUint64,
		elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y),
		//big.NewInt(0),
		nil,
	)
	fmt.Println(acc)
}
