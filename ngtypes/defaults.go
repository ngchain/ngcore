package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"
)

// FIXME: before init network should manually init PK & Sign
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
		genesisGenerateTxSign, _ := hex.DecodeString("c79d434e9070edaea3e1c5cb59c7c19e5c777a0a6130a5997a634962ce86d8fd0af94408853ff856b5fcc88bf76c3383d43ce1134da7c93a877960bb1b53caa6")
		return genesisGenerateTxSign
	case NetworkType_TESTNET:
		genesisGenerateTxSign, _ := hex.DecodeString("75179201d03e7c66703cf4570b2e1e6ae23caa1fd545fdac9f28dcb6433d5a2c4b48fbfcc0b4e996e446212b65cb39c94e3ec2f18fc8d16baffada7d5a9e9301")
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
		genesisBlockNonce, _ := hex.DecodeString("26cab897baa3cd74")
		return genesisBlockNonce
	case NetworkType_TESTNET:
		genesisBlockNonce, _ := hex.DecodeString("bfded3fdcf6b5b91")
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
		y, m, _ := time.Now().Date()
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).Unix() // first day of the month by default
	case NetworkType_TESTNET:
		return time.Date(2020, time.July, 28, 14, 0, 0, 0, time.UTC).Unix()
	case NetworkType_MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}
