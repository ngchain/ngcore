package message_test

import (
	"encoding/hex"
	"github.com/ngchain/ngcore/ngp2p/message"
	"testing"

	"github.com/ngchain/ngcore/utils"
)

func TestMsgUnmarshal(t *testing.T) {
	raw, _ := hex.DecodeString("0a950108ffffffffffffffffff011210bea85b3bee364c8e8e32d891b2ff2e2e180120adf498f6052a2508021221029c0bdf5530345b2a6328a49563dd466c57c36391c47ae975fa0d85aa1e8c33b5324730450221008e6d8d0cf613f93e539768412be6804cf927282e676fc7626984cc80a6f5d1bd0220304e4a5303db7e41e6f1334ded61cb1f234d641141bd2572b7bbaefc47a176fa12221a20482960513c5801b96e57ae1e47c1c8c621216bcb9e189a1b188cc445b526f3e3")
	// rbw. _ := hex.DecodeString("0a950108ffffffffffffffffff011210a715eeadbce1443ea780b523509e395d180120fcf898f6052a2508021221029c0bdf5530345b2a6328a49563dd466c57c36391c47ae975fa0d85aa1e8c33b53247304502210084e98865d550b8abdb2dbeb8a5dcfe1b80ba77d41f4d76059f7a701a143f400502200336cd967637428d45470c7866fae3850ec1fbcb52d417c884cae86e1900ae3512221a20482960513c5801b96e57ae1e47c1c8c621216bcb9e189a1b188cc445b526f3e3")
	msg := new(message.Message)
	err := utils.Proto.Unmarshal(raw, msg)
	if err != nil {
		t.Error(err)
	}

	t.Log(msg)
}
