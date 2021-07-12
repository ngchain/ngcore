package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"
)

// GenesisAddressBase58 is the genesis address in base58 str
// FIXME: before initializing new network, should manually init PK & Sign
// 	use genesistool tool to check and generate valid values
const (
	GenesisAddressBase58 = "2Reeyjud1sS7Pq5BMtsK3pSmDnJPfaz4jiffTWwmJdzBvTbq"
)

// decoded genesis variables
var (
	GenesisAddress, _ = NewAddressFromBS58(GenesisAddressBase58)
	AvailableNetworks = []Network{
		ZERONET,
		TESTNET,
	}
)

// PoW const
const (
	// MinimumDifficulty is the minimum of pow minimumDifficulty because my laptop has 200 h/s, I believe you can either
	minimumDifficulty = 200 << 4 // Target = MaxTarget / diff
	TargetTime        = 16 * time.Second
	BlockCheckRound   = 10 // do converge if fall behind one round

	MatureRound  = 10                            // not mandatory required, can be modified by different daemons
	MatureHeight = MatureRound * BlockCheckRound // just for calculating the immature balance
)

// PoW variables
var (
	minimumBigDifficulty = big.NewInt(minimumDifficulty)
	// MaxTarget is the Max value of mining target
	MaxTarget = new(big.Int).SetBytes([]byte{
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	})
)

// Maximum sizes
const (
	// TxMaxExtraSize 1 << 20 = 1024K = 1M, if more than 1m, extra should be separated and using more than one append
	TxMaxExtraSize = 1 << 20
	// TimestampSize is The length of a timestamp bytes
	TimestampSize = 8
	// HashSize is the length of a hash bytes
	HashSize = 32
	// DiffSize is the length of a difficulty
	DiffSize = 32
	// NonceSize is the length of a nonce bytes
	NonceSize = 8 // nonce uses 8 bytes

	// PrivSize is the length of one private key in bytes
	PrivSize = 32
	// AddressSize some for tx
	AddressSize = 35
	// SignatureSize is the size used by signature and is 64 bytes(R 32 + S 32)
	SignatureSize = 64
)

// Unit const
const (
	FloatNG = 1_000_000_000_000_000_000.0
	pico    = 1_000_000_000_000_000_000 // 10^(-18)
)

// Units variables:
// https://en.wikipedia.org/wiki/Unit_prefix
// https://en.wikipedia.org/wiki/Metric_prefix
var (
	NG       = new(big.Int).SetUint64(pico)
	NGSymbol = "NG"
	// picoNG       = big.NewInt(1)
	// picoNGSymbol = "pNG"
)

// GetEmptyHash return an empty hash
func GetEmptyHash() []byte {
	return make([]byte, HashSize)
}

func GetGenesisGenerateTxSignature(network Network) []byte {
	switch network {
	case ZERONET:
		genesisGenerateTxSign, _ := hex.DecodeString("4994ae32b1e6665f2bcf2231e1d11785e3e177d79f8eacfe19e1e3267c94da8c5aac5df3a7748ba834f2ffb8d76655fd65dde3fe7425ca6e43d5b047cec373f7")
		return genesisGenerateTxSign
	case TESTNET:
		genesisGenerateTxSign, _ := hex.DecodeString("ca92849de30ba67ceb9586c3781e78504de37dc8c591056e0048ac1b62ed322f693f87159f09ed7c2468a2e47aa3bc9f963b4b43335342ef277b7ea1b550c58a")
		return genesisGenerateTxSign
	case MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

func GetGenesisBlockNonce(network Network) []byte {
	switch network {
	case ZERONET:
		genesisBlockNonce, _ := hex.DecodeString("1d0b4960bc05828a")
		return genesisBlockNonce
	case TESTNET:
		genesisBlockNonce, _ := hex.DecodeString("628d06d30f4c8e49")
		return genesisBlockNonce
	case MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

// GetGenesisTimestamp returns the genesis timestamp
// must be the time chain started, or the difficulty algo wont work
// FIXME: should be the time network starts
func GetGenesisTimestamp(network Network) uint64 {
	switch network {
	case ZERONET:
		return uint64(time.Date(2020, time.October, 24, 0, 0, 0, 0, time.UTC).Unix())
	case TESTNET:
		return uint64(time.Date(2020, time.November, 11, 11, 11, 11, 11, time.UTC).Unix())
	case MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

// GetMatureHeight will return the next mature height for now
//  it is 100 * X
func GetMatureHeight(currentHeight uint64) uint64 {
	if currentHeight < MatureHeight {
		return 0
	}

	return currentHeight / MatureHeight * MatureHeight
}
