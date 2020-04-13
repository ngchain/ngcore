package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/mr-tron/base58"
)

const (
	NetworkID = testnetNetworkID // for hard fork

	mainnetNetworkID = 1

	testnetNetworkID = -1
)

// FIXME: before init network should manually init PK & Sign
var (
	GenesisPublicKeyBase58 = "v9fATcLJhipGXGGeyKixVrdRpFYuvHAonA2EjDfEni1g"
	GenesisPublicKey, _    = base58.FastBase58Decoding(GenesisPublicKeyBase58)

	GenesisGenerateTxSignBase58 = "5kVQcqFLNiqQxC7UL8E9wwX9csbD5MGJ2vSuvPZWU8BxfZtDLz7HaUhwaCFwsbFd4GTKC4AEbbChJp18VZa82uTE"
	GenesisGenerateTxSign, _    = base58.FastBase58Decoding(GenesisGenerateTxSignBase58)
)

var (
	// MinimumDifficulty is the minimum of pow difficulty because my laptop has 50 h/s, I believe you can either
	MinimumDifficulty = big.NewInt(50 * 10)
	MaxTarget         = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}) // new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0)) // Target = MaxTarget / diff
	GenesisTarget     = new(big.Int).Div(MaxTarget, MinimumDifficulty)

	GenesisNonceBytes, _ = hex.DecodeString("74ba15b9b7bc1df3")
	GenesisNonce         = new(big.Int).SetBytes(GenesisNonceBytes)

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

var (
	BlockMaxTxsSize = 1 << 25 // 32M
)

// PoW
const (
	TargetTime      = 12 * time.Second
	BlockCheckRound = 10
)

// Units
var (
	FloatNG        = 1000000.0
	MegaNG         = new(big.Int).Mul(NG, big.NewInt(1000000))
	MegaNGSymbol   = "MNG"
	NG             = new(big.Int).SetUint64(1000000)
	NGSymbol       = "NG"
	MicroNG        = GetBig1()
	MicroNGSymbol  = "μNG"
	OneBlockReward = new(big.Int).Mul(NG, big.NewInt(10)) // 10NG
)

func GetBig0() *big.Int {
	return big.NewInt(0)
}

func GetBig0Bytes() []byte {
	return big.NewInt(0).Bytes()
}

func GetBig1() *big.Int {
	return big.NewInt(1)
}
