package ngtypes

import (
	"math/big"
	"testing"

	"github.com/c0mm4nd/rlp"
)

// FuzzTxDecode hammers the wire-decoding path a peer controls: rlp
// bytes into a FullTx, then every stateless accessor. Reject with
// errors, never panic
func FuzzTxDecode(f *testing.F) {
	key, _ := GenerateKey()
	tx := NewUnsignedTx(ZERONET, TransactTx, 1, NewAddress(key), big.NewInt(1), big.NewInt(0), nil)
	_ = tx.Signature(key)
	if raw, err := rlp.EncodeToBytes(tx); err == nil {
		f.Add(raw)
	}
	f.Add([]byte{0xc0})
	f.Add([]byte{0xf8, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		var tx FullTx
		if err := rlp.DecodeBytes(data, &tx); err != nil {
			return
		}
		if tx.Value == nil || tx.Fee == nil {
			return // rlp may leave big.Ints nil; constructors never do
		}
		_ = tx.Verify(nil)
		_, _ = tx.From()
		_ = tx.IsCompactEnvelope()
		_ = tx.EnvelopeScheme()
		_ = tx.GetHash()
	})
}

// FuzzBlockDecode: rlp bytes into a FullBlock, then the full stateless
// CheckError (pow, caps, tries, witness)
func FuzzBlockDecode(f *testing.F) {
	if raw, err := rlp.EncodeToBytes(GetGenesisBlock(ZERONET)); err == nil {
		f.Add(raw)
	}
	f.Add([]byte{0xc0})

	f.Fuzz(func(t *testing.T, data []byte) {
		var block FullBlock
		if err := rlp.DecodeBytes(data, &block); err != nil {
			return
		}
		if block.BlockHeader == nil {
			return
		}
		_ = block.CheckError()
		_ = block.GetHash()
	})
}

// FuzzCommitCode: the commit-code decoder faces attacker bytes at
// commit validation — deflate bombs and corrupt tags must error, not
// panic; the inflate is size-bounded
func FuzzCommitCode(f *testing.F) {
	f.Add(EncodeCommitCode([]byte{0x00, 0x61, 0x73, 0x6d}))
	f.Add([]byte{0x00})
	f.Add([]byte{0x01})
	f.Add([]byte{0x01, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeCommitCode(data)
	})
}
