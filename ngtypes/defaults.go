package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"
)

// FIXME: before init network should manually init PK & Sign
// use `go run ./cmd/ngcore gentools check` check and generate valid values
const (
	GenesisAddressBase58 = "Jqc3bB6vtsDSfeuewG2fskvCkEXcpqGz9u2h4P4wFWsPDe7g"
)

// decoded genesis variables
var (
	//Network                   = NetworkType_TESTNET // can be changed by arg FIXME: set to mainnet when releasing
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

	// GenesisTimestamp must be the time chain started, or the difficulty algo wont work
	GenesisTimestamp = time.Date(2020, time.July, 28, 14, 0, 0, 0, time.UTC).Unix()
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
	FloatNG    = 1000000.0
	mega       = 1000000
	OneBlockNG = 10
)

// Units variables
var (
	MegaNG            = new(big.Int).Mul(NG, big.NewInt(mega))
	MegaNGSymbol      = "MNG"
	NG                = new(big.Int).SetUint64(mega)
	NGSymbol          = "NG"
	MicroNG           = GetBig1()
	MicroNGSymbol     = "μNG"
	OneBlockBigReward = new(big.Int).Mul(NG, big.NewInt(OneBlockNG)) // 10NG
)

// GetEmptyHash return an empty hash
func GetEmptyHash() []byte {
	return make([]byte, HashSize)
}

func GetGenesisGenerateTxSignature(network NetworkType) []byte {
	switch network {
	case NetworkType_ZERONET:
		genesisGenerateTxSign, _ := hex.DecodeString("2f06927456808d85ef71c6ff35d1cbacf6dfafabb1a8f0155716361735413c4f917ee3438be130f505e43c8d3ce64442d32878df4113d496c2f6f2c51aae7e2d")
		return genesisGenerateTxSign
	case NetworkType_TESTNET:
		genesisGenerateTxSign, _ := hex.DecodeString("bbef197b1c74a762390bf37a7e17830e0e845239937dece90c09d64a9e82a3e8b683ad41ebb6a879c14cbf2e8070c3b1b5cbd1c32da2fcc0a4a637d572858a8d")
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
		genesisBlockNonce, _ := hex.DecodeString("52ef544b2f8fe12f")
		return genesisBlockNonce
	case NetworkType_TESTNET:
		genesisBlockNonce, _ := hex.DecodeString("4530ef8acd530abc")
		return genesisBlockNonce
	case NetworkType_MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}
