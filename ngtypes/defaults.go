package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/mr-tron/base58"
)

const (
	Network = testnetNetworkID // for hard fork

	mainnetNetworkID = 1

	testnetNetworkID = -1
)

// FIXME: before init network should manually init PK & Sign
const (
	GenesisPublicKeyBase58      = "ruBBKVQgTKDaB8dFbSZqQeJkgnZxzL26s8gwatw8M1F5"
	GenesisGenerateTxSignBase58 = "3kCnakJZV9yYiFXc4dgDFBTp7KgZPdDLvsjqSux75FvsaroyTa7Xx4ksWk3gk2QS1zZELD15omcfrrQDVUuu6BmZ"
	GenesisBlockNonceHex        = "f8ed39a3a407bc4d"
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
	TargetTime      = 1 * time.Minute
	BlockCheckRound = 10
)

// PoW variables
var (
	// MinimumDifficulty is the minimum of pow difficulty because my laptop has 50 h/s, I believe you can either
	minimumDifficulty = big.NewInt(50 * 60 * 20)
	// Target = MaxTarget / diff
	maxTarget = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
	genesisTarget = new(big.Int).Div(maxTarget, minimumDifficulty)

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

// Maximum sizes
var (
	BlockMaxTxsSize = 1 << 25 // 32M
	TxMaxExtraSize  = 1 << 20 // if more than 1m, extra should be separated ot multi append
	NonceSize       = 8       // nonce uses 8 bytes
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

// GetBig0 returns a new big 0.
func GetBig0() *big.Int {
	return big.NewInt(0)
}

// GetBig0Bytes returns a new big 0's bytes.
func GetBig0Bytes() []byte {
	return big.NewInt(0).Bytes()
}

// GetBig1 returns a new big 1.
func GetBig1() *big.Int {
	return big.NewInt(1)
}
