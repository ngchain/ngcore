package ngtypes

import (
	"math/big"
	"time"

	"github.com/mr-tron/base58"
)

var GenesisPublicKeyBase58 = "MubnecPG2WYJGen2LtKqnmt3qCrP3aeRTeXZNiiPVdoZgmaCNFsRythAkiX7xAaP1LFp1RcYsKzfwQXTnphB2SSi"
var GenesisPublicKey, _ = base58.FastBase58Decoding(GenesisPublicKeyBase58)

const (
	Version   = -1
	NetworkID = -1
)

var (
	// MinimumDifficulty is the minimum of pow difficulty because my laptop has 50 h/s, I believe you can either
	MinimumDifficulty = big.NewInt(50 * 10)
	MaxTarget         = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}) // new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0)) // Target = MaxTarget / diff
	GenesisTarget     = new(big.Int).Div(MaxTarget, MinimumDifficulty)
	GenesisNonce      = new(big.Int).SetUint64(0)

	genesisTimestamp = time.Date(2020, time.February, 2, 2, 2, 2, 2, time.UTC).Unix()
)

func GetBig0() *big.Int {
	return big.NewInt(0)
}

func GetBig0Bytes() []byte {
	return big.NewInt(0).Bytes()
}

func GetBig1() *big.Int {
	return big.NewInt(1)
}

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
	MicroNG        = GetBig1()
	MicroNGSymbol  = "μNG"
	OneBlockReward = new(big.Int).Mul(NG, big.NewInt(10)) // 10NG
)
