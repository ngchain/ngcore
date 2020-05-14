package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/mr-tron/base58"
)

// network configure
const (
	Network        = testnetNetwork  // for hard fork
	mainnetNetwork = 1               // mainnet shall be positive
	testnetNetwork = -mainnetNetwork // testnet shall be neg
)

// FIXME: before init network should manually init PK & Sign
const (
	GenesisPublicKeyBase58      = "ruBBKVQgTKDaB8dFbSZqQeJkgnZxzL26s8gwatw8M1F5"
	GenesisGenerateTxSignBase58 = "3kCnakJZV9yYiFXc4dgDFBTp7KgZPdDLvsjqSux75FvsaroyTa7Xx4ksWk3gk2QS1zZELD15omcfrrQDVUuu6BmZ"
	GenesisBlockNonceHex        = "ffb4e186e415927b"
)

// decoded genesis variables
var (
	GenesisPublicKey, _       = base58.FastBase58Decoding(GenesisPublicKeyBase58)
	GenesisGenerateTxSign, _  = base58.FastBase58Decoding(GenesisGenerateTxSignBase58)
	genesisBlockNonceBytes, _ = hex.DecodeString(GenesisBlockNonceHex)
	genesisBlockNonce         = new(big.Int).SetBytes(genesisBlockNonceBytes)
)

// PoW const
const (
	// MinimumDifficulty is the minimum of pow difficulty because my laptop has 50 h/s, I believe you can either
	difficulty      = 50 * 10 // Target = MaxTarget / diff
	TargetTime      = 10 * time.Second
	BlockCheckRound = 10
)

// PoW variables
var (
	minimumBigDifficulty = big.NewInt(difficulty)
	MaxTarget            = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

// Maximum sizes
var (
	BlockMaxTxsSize = 1 << 25 // 32M
	TxMaxExtraSize  = 1 << 20 // if more than 1m, extra should be separated ot multi append

	TimestampSize = 8
	HashSize      = 32
	NonceSize     = 8 // nonce uses 8 bytes
)

// Unit consts
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
