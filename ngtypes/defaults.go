package ngtypes

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/pkg/errors"
)

// GenesisAddressBase58 is the genesis address in base58 str: the
// all-zero 32-byte address ('1' is the base58 digit for a zero byte).
// No public key hashes to it, so it doubles as the burn address
const (
	GenesisAddressBase58 = "11111111111111111111111111111111"
)

// decoded genesis variables
var (
	GenesisAddress    = mustAddressFromBS58(GenesisAddressBase58)
	AvailableNetworks = []Network{
		ZERONET,
		TESTNET,
	}
)

// PoW const
const (
	// MinimumDifficulty is the minimum of pow minimumDifficulty because my laptop has 200 h/s, I believe you can either
	minimumDifficulty = 200 << 4        // Target = MaxTarget / diff
	TargetTime        = 4 * time.Second // fast blocks, but above global propagation delay to bound the orphan rate
	BlockCheckRound   = 10              // do converge if fall behind one round

	// MaxUncles caps how many uncle (GHOST) references a block may carry.
	// UncleMaxDepth bounds how many generations back an uncle's fork point
	// may be; it must stay below BlockCheckRound so the referenced side
	// blocks still live above the finality/pruning line.
	MaxUncles     = 2
	UncleMaxDepth = 6

	// TimestampDriftTolerance bounds how far (MILLISECONDS) a block
	// timestamp may run ahead of the local clock. Block timestamps are
	// unix-milliseconds so the 1s target is reachable from below
	TimestampDriftTolerance = uint64(15 * time.Second / time.Millisecond)

	MatureRound  = 10                            // not mandatory required, can be modified by different daemons
	MatureHeight = MatureRound * BlockCheckRound // just for calculating the immature balance

	// MaxBlockTxCount caps how many txs one block may carry — a
	// CONSENSUS rule, not miner etiquette: oversized blocks are invalid
	MaxBlockTxCount = 512
	// MaxBlockBytes caps a block's total serialized size (8 MiB), the
	// hard blowup guard whatever the envelope sizes are
	MaxBlockBytes = 1 << 23

	// MaxBlockGas caps the TOTAL contract-execution toll one block may
	// consume (8 full per-call budgets; hundreds of typical calls).
	// Execution is deterministic, so every node draws the same line:
	// runs past the budget are skipped, recorded as such in receipts
	MaxBlockGas = 1 << 27

	// MaxContractSourceSize caps a contract's source text: activation
	// compiles it inside block validation, so its cost must be bounded
	MaxContractSourceSize = 256 << 10
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
	// AddressSize is the byte length of an Address: the keccak-256
	// hash of the owner's public key (see AddressOfPubKey). Hashing
	// keeps public keys off chain until spend time and makes the large
	// post-quantum keys usable as compact addresses
	AddressSize = 32
	// SignatureSize is the size used by signature and is 64 bytes(R 32 + S 32)
	SignatureSize = 64
)

var ErrHashSize = errors.New("the length of hash is wrong ")

// Unit const: NG is an 18-decimal coin; money never touches a float —
// human surfaces carry decimal strings, the chain carries big.Int
const (
	pico = 1_000_000_000_000_000_000 // raw units per NG (10^18)
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
	case TESTNET, ZERONET:
		genesisGenerateTxSign, _ := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
		return genesisGenerateTxSign
	case MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

func GetGenesisBlockNonce(network Network) []byte {
	switch network {
	case ZERONET, TESTNET:
		genesisBlockNonce, _ := hex.DecodeString("0000000000000000")
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
		return uint64(time.Date(2020, time.October, 24, 0, 0, 0, 0, time.UTC).UnixMilli())
	case TESTNET:
		return uint64(time.Date(2020, time.November, 11, 11, 11, 11, 11, time.UTC).UnixMilli())
	case MAINNET:
		panic("not ready for mainnet")
	default:
		panic("unknown network")
	}
}

// GetMatureHeight will return the next mature height for now
//
//	it is 100 * X
func GetMatureHeight(currentHeight uint64) uint64 {
	if currentHeight < MatureHeight {
		return 0
	}

	return currentHeight / MatureHeight * MatureHeight
}
