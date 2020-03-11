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
	GenesisHash    = "123"
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

	Empty32 = make([]byte, 32)
	Empty8  = make([]byte, 8)
	Empty4  = make([]byte, 4)

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

const (
	TargetTime = 12 * time.Second
	CheckRound = 8
)

// Units
var (
	OneNGIN = new(big.Int).SetUint64(1000000)
	//MinimalUnit = big.NewInt(1)
	OneBlockReward = new(big.Int).Mul(OneNGIN, big.NewInt(10)) // 10NG
)
