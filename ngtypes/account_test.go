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
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Error(err)
	}

	randUint64 := fastrand.Uint64n(math.MaxUint64)
	acc := NewAccount(
		randUint64,
		elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y),
		//big.NewInt(0),
		nil,
	)
	fmt.Println(acc)
}
