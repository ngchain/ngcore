package ngtypes

import (
	"fmt"
	"math"
	"testing"

	"github.com/NebulousLabs/fastrand"
	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/utils"
)

// TestNewAccount is testing func NewAccount
func TestNewAccount(t *testing.T) {
	privateKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		log.Error(err)
	}

	randUint64 := fastrand.Uint64n(math.MaxUint64)
	acc := NewAccount(
		randUint64,
		utils.PublicKey2Bytes(*privateKey.PubKey()),
		// big.NewInt(0),
		nil,
	)
	fmt.Println(acc)
}
