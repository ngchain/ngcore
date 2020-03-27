package ngtypes

import (
	"encoding/hex"
	"math"
	"math/big"
	"time"
)

const (
	GenesisBalance = math.MaxInt64
	GenesisData    = "NGIN TESTNET"
)

var GenesisPK, _ = hex.DecodeString("041826d860840c977c9566ac5f24d620d7edfaa51285091e3456fd5b60ccf9e5727a646e33f5d9c85a98491d88c65eafd04119c698ee4c7869b240801cc5bb2d86")

const (
	Version   = -1
	NetworkId = -1
)

var (
	MaxTarget     = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}) // new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0)) // Target = MaxTarget / diff
	GenesisTarget = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
	GenesisNonce  = new(big.Int).SetUint64(0)
	Big0          = big.NewInt(0)
	Big0Bytes     = make([]byte, 0) // not nil
	Big1          = big.NewInt(1)
	Big1Bytes     = []byte{1}

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

var (
	BlockMaxTxsSize = 1 << 25 // 32M
)

// PoW
const (
	TargetTime      = 12 * time.Second
	BlockCheckRound = 10
	VaultCheckRound = 3
)

// Units
var (
	FloatNG        = 1000000.0
	MegaNG         = new(big.Int).Mul(NG, big.NewInt(1000000))
	MegaNGSymbol   = "MNG"
	NG             = new(big.Int).SetUint64(1000000)
	NGSymbol       = "NG"
	MicroNG        = Big1
	MicroNGSymbol  = "μNG"
	OneBlockReward = new(big.Int).Mul(NG, big.NewInt(10)) // 10NG
)
