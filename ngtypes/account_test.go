package ngtypes_test

import (
	"testing"

	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("ngtypes_test")

// TestNewAccount is testing func NewAccount.
func TestNewAccount(t *testing.T) {
	privateKey, err := ngtypes.GenerateKey()
	if err != nil {
		log.Error(err)
	}

	addr := ngtypes.NewAddress(privateKey)

	acc := ngtypes.NewAccount(addr, nil, nil)
	t.Log(acc)
}

func TestJSONAccount(t *testing.T) {
	key, _ := ngtypes.GenerateKey()
	account1 := ngtypes.NewAccount(ngtypes.NewAddress(key), []byte("(module)"), nil)
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

	if eq, _ := account1.Equals(account2); !eq {
		t.Errorf("account \n 2 %#v \n is different from \n 1 %#v", account2, account1)
	}

	if eq, _ := account1.Equals(account2); !eq {
		t.Errorf("account \n 2 %#v \n is different from \n 1 %#v", account2, account1)
	}
}
