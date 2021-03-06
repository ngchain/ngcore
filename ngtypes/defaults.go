package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"
)

// FIXME: before initializing new network, should manually init PK & Sign
// use `go run ./cmd/ngcore gentools check` check and generate valid values
const (
	GenesisAddressBase58 = "QVSdpMLFwUtECb3SxgLt8YeQwkHGmzh5ZexjGCUB2E5koFhJ"
)

// decoded genesis variables
var (
	GenesisAddress, _ = NewAddressFromBS58(GenesisAddressBase58)
	AvailableNetworks = []NetworkType{NetworkType_ZERONET, NetworkType_TESTNET}
)

// PoW const
const (
	// MinimumDifficulty is the minimum of pow minimumDifficulty because my laptop has 200 h/s, I believe you can either
	minimumDifficulty = 200 << 4         // Target = MaxTarget / diff
	TargetTime        = 16 * time.Second // change time from 10 -> 16 = 1 << 4
	BlockCheckRound   = 10               // do fork if fall behind one round
)

// PoW variables
var (
	minimumBigDifficulty = big.NewInt(minimumDifficulty)
	// Max Value of Target
	MaxTarget = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
)

// Maximum sizes
const (
	// !NO MAX LIMITATION!
	//BlockMaxTxsSize = 1 << 25 // 32M
	TxMaxExtraSize = 1 << 20 // if more than 1m, extra should be separated ot multi append
	// The length of a timestemp bytes
	TimestampSize = 8
	// The length of a hash bytes
	HashSize = 32
	// The length of a nonce bytes
	NonceSize = 8 // nonce uses 8 bytes

	// some for tx
	AddressSize   = 35
	SignatureSize = 64 // signature uses 64 bytes, R 32 & S 32
)

// Unit const
const (
	FloatNG = 1_000_000_000_000_000_000.0
	pico    = 1_000_000_000_000_000_000 // 10^(-18)
)

// Units variables:
//https://en.wikipedia.org/wiki/Unit_prefix
//https://en.wikipedia.org/wiki/Metric_prefix
var (
	NG           = new(big.Int).SetUint64(pico)
	NGSymbol     = "NG"
	picoNG       = big.NewInt(1)
	picoNGSymbol = "pNG"
)

// GetEmptyHash return an empty hash
func GetEmptyHash() []byte {
	return make([]byte, HashSize)
}

func GetGenesisGenerateTxSignature(network NetworkType) []byte {
	switch network {
	case NetworkType_ZERONET:
		genesisGenerateTxSign, _ := hex.DecodeString("1aca22bb998d0bea643f75c126b8be259839aa4c2c13829d737c57c8f20371edbc7014a79e2af97e8119c92fcc9f4642c5f42639cad59429fbc4336ee8dcc858")
		return genesisGenerateTxSign
	case NetworkType_TESTNET:
		genesisGenerateTxSign, _ := hex.DecodeString("5ca0c8099874dd61b4ebbfb6e984f5f1e7f6287d1093f05d3ed973a5fb3f3352bf7fc3c78d93dcaf077f98602338445e4187ae5f225a2d79ff9b36ec8c61b98a")
		return genesisGenerateTxSign
	case NetworkType_MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

func GetGenesisBlockNonce(network NetworkType) []byte {
	switch network {
	case NetworkType_ZERONET:
		genesisBlockNonce, _ := hex.DecodeString("c800120f3ae9a2fc")
		return genesisBlockNonce
	case NetworkType_TESTNET:
		genesisBlockNonce, _ := hex.DecodeString("115c488d6d09dc41")
		return genesisBlockNonce
	case NetworkType_MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

// GenesisTimestamp must be the time chain started, or the difficulty algo wont work
// FIXME: should be the time network starts
func GetGenesisTimestamp(network NetworkType) int64 {
	switch network {
	case NetworkType_ZERONET:
		return time.Date(2020, time.October, 24, 0, 0, 0, 0, time.UTC).Unix()
	case NetworkType_TESTNET:
		return time.Date(2020, time.November, 11, 11, 11, 11, 11, time.UTC).Unix()
	case NetworkType_MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}
