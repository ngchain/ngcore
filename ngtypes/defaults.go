package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"
)

const (
	ZERONET = 0
	TESTNET = 1
	MAINNET = 2
)

// GenesisAddressBase58 is the genesis address in base58 str
// FIXME: before initializing new network, should manually init PK & Sign
// 	use `go run ./cmd/ngcore gentools check` check and generate valid values
const (
	GenesisAddressBase58 = "QVSdpMLFwUtECb3SxgLt8YeQwkHGmzh5ZexjGCUB2E5koFhJ"
)

// decoded genesis variables
var (
	GenesisAddress, _ = NewAddressFromBS58(GenesisAddressBase58)
	AvailableNetworks = []uint8{
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
	MaxTarget = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
)

// Maximum sizes
const (
	// TxMaxExtraSize 1 << 20 = 1024K = 1M, if more than 1m, extra should be separated and using more than one append
	TxMaxExtraSize = 1 << 20
	// TimestampSize is The length of a timestamp bytes
	TimestampSize = 8
	// HashSize is the length of a hash bytes
	HashSize = 32
	// NonceSize is the length of a nonce bytes
	NonceSize = 8 // nonce uses 8 bytes

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

func GetGenesisGenerateTxSignature(network uint8) []byte {
	switch network {
	case ZERONET:
		genesisGenerateTxSign, _ := hex.DecodeString("1aca22bb998d0bea643f75c126b8be259839aa4c2c13829d737c57c8f20371edbc7014a79e2af97e8119c92fcc9f4642c5f42639cad59429fbc4336ee8dcc858")
		return genesisGenerateTxSign
	case TESTNET:
		genesisGenerateTxSign, _ := hex.DecodeString("5ca0c8099874dd61b4ebbfb6e984f5f1e7f6287d1093f05d3ed973a5fb3f3352bf7fc3c78d93dcaf077f98602338445e4187ae5f225a2d79ff9b36ec8c61b98a")
		return genesisGenerateTxSign
	case MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

func GetGenesisBlockNonce(network uint8) []byte {
	switch network {
	case ZERONET:
		genesisBlockNonce, _ := hex.DecodeString("84e1be18c794f125")
		return genesisBlockNonce
	case TESTNET:
		genesisBlockNonce, _ := hex.DecodeString("55e0414311982f0e")
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
func GetGenesisTimestamp(network uint8) uint64 {
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
