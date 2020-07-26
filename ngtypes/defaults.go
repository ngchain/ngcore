package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"
)

// FIXME: before init network should manually init PK & Sign
// try `go test ./...`  until all passed
const (
	NETWORK                  = NetworkType_TESTNET
	GenesisAddressBase58     = "Jqc3bB6vtsDSfeuewG2fskvCkEXcpqGz9u2h4P4wFWsPDe7g"
	GenesisGenerateTxSignHex = "bbef197b1c74a762390bf37a7e17830e0e845239937dece90c09d64a9e82a3e8b683ad41ebb6a879c14cbf2e8070c3b1b5cbd1c32da2fcc0a4a637d572858a8d"
	GenesisBlockNonceHex     = "92794d9c9c69dae5"
)

// decoded genesis variables
var (
	GenesisAddress, _         = NewAddressFromBS58(GenesisAddressBase58)
	GenesisGenerateTxSign, _  = hex.DecodeString(GenesisGenerateTxSignHex)
	genesisBlockNonceBytes, _ = hex.DecodeString(GenesisBlockNonceHex)
	genesisBlockNonce         = new(big.Int).SetBytes(genesisBlockNonceBytes)
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
	MaxTarget            = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

// Maximum sizes
var (
	// !NO MAX LIMITATION!
	//BlockMaxTxsSize = 1 << 25 // 32M
	TxMaxExtraSize = 1 << 20 // if more than 1m, extra should be separated ot multi append

	TimestampSize = 8
	HashSize      = 32
	NonceSize     = 8 // nonce uses 8 bytes
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
