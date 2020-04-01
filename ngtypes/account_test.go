package ngtypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math"
	"testing"

	"github.com/NebulousLabs/fastrand"

	"github.com/ngchain/ngcore/utils"
)

func TestNewAccount(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Error(err)
	}

	randUint64 := fastrand.Uint64n(math.MaxUint64)
	acc := NewAccount(
		randUint64,
		utils.ECDSAPublicKey2Bytes(privateKey.PublicKey),
		// big.NewInt(0),
		nil,
	)
	fmt.Println(acc)
}
