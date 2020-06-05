package ngtypes_test

import (
	"math"
	"testing"

	"github.com/NebulousLabs/fastrand"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/secp256k1"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("ngtypes_test")

// TestNewAccount is testing func NewAccount.
func TestNewAccount(t *testing.T) {
	privateKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		log.Error(err)
	}

	randUint64 := fastrand.Uint64n(math.MaxUint64)
	acc := ngtypes.NewAccount(
		randUint64,
		utils.PublicKey2Bytes(*privateKey.PubKey()),
		// big.NewInt(0),
		nil,
		nil,
	)
	t.Log(acc)
}

func TestJSONAccount(t *testing.T) {
	account1 := ngtypes.GetGenesisStyleAccount(1)
	jsonBlock, err := utils.JSON.Marshal(account1)
	if err != nil {
		t.Error(err)
		return
	}

	t.Log(string(jsonBlock))

	account2 := &ngtypes.Account{}
	err = utils.JSON.Unmarshal(jsonBlock, account2)
	if err != nil {
		t.Error(err)
		return
	}

	if !proto.Equal(account1, account2) {
		t.Errorf("account 2 %v is different from 1 %v", account2, account1)
	}
}
