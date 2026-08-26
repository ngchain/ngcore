package ngtypes

import (
	"bytes"
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/cbergoon/merkletree"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

// Errors for Commitment
var (
	ErrCommitHashInvalid   = errors.New("commitment hash must be 32 bytes")
	ErrCommitFeeInvalid    = errors.New("commitment fee is nil")
	ErrCommitHeightInvalid = errors.New("commitment height does not match its block")
	ErrCommitUnsigned      = errors.New("unsigned commitment")
	ErrCommitSignInvalid   = errors.New("commitment signature is invalid")
)

// Commitment is the blind half of the mandatory commit-reveal private
// mempool: the committer publishes blake3(revealTx.UnheightedHash() ‖ Salt)
// in one block, hiding the tx's content, then reveals & executes the effect
// tx (Transact/Deploy/Destroy) in a LATER block. It is NOT a TxType — it
// rides a block's own Commits list, folded into the tx-trie root. Keyed by
// its Hash (unique per tx+salt), so there is no per-address slot to grief.
type Commitment struct {
	Network Network
	Height  uint64   // the block height this commitment is packed at (like a tx's height-lock)
	Hash    []byte   // 32B = blake3(revealTx.UnheightedHash() ‖ Salt)
	Fee     *big.Int // small anti-spam fee, charged to the committer on inclusion
	Sign    []byte   `rlp:"optional"`
}

// NewCommitment builds an unsigned commitment over a 32-byte reveal hash.
func NewCommitment(network Network, height uint64, hash []byte, fee *big.Int) *Commitment {
	if fee == nil {
		fee = big.NewInt(0)
	}

	return &Commitment{
		Network: network,
		Height:  height,
		Hash:    hash,
		Fee:     fee,
	}
}

// IsSigned reports whether the commitment carries a signature envelope.
func (c *Commitment) IsSigned() bool {
	return len(c.Sign) != 0
}

// GetUnsignedHash returns blake3(rlp(commitment without Sign)); this is
// what the committer signs and what the merkle trie commits.
func (c *Commitment) GetUnsignedHash() []byte {
	sign := c.Sign
	c.Sign = nil
	raw, err := rlp.EncodeToBytes(c)
	if err != nil {
		panic(err)
	}

	c.Sign = sign
	return utils.Hash256(raw)
}

// SigningHash is the digest the committer signs. Like a reveal's SigningHash it
// EXCLUDES both Sign and Height, so one signature stays valid at whatever height
// the commitment is finally packed at — a node may relay a commitment to a later
// block (if it missed its target) without a fresh signature. Height cannot be
// abused: a block requires each commitment's Height to equal its own, the reveal
// window is measured relative to wherever the commitment lands, and a commitment
// hash may be recorded on chain only once, so it cannot be double-charged.
func (c *Commitment) SigningHash() []byte {
	sign, height := c.Sign, c.Height
	c.Sign, c.Height = nil, 0
	raw, err := rlp.EncodeToBytes(c)
	if err != nil {
		panic(err)
	}

	c.Sign, c.Height = sign, height
	return utils.Hash256(raw)
}

// envelope splits Sign into the scheme, public key (resolving the compact
// form through the registry) and the signature, EXACTLY like FullTx.envelope.
func (c *Commitment) envelope(lookup PubKeyResolver) (scheme SigScheme, pubKey, sig []byte, err error) {
	if len(c.Sign) < 2 {
		return 0, nil, nil, ErrCommitUnsigned
	}

	scheme = SigScheme(c.Sign[1])
	pkLen, sigLen := PubKeySize(scheme), SigSize(scheme)
	if pkLen == 0 {
		return 0, nil, nil, errors.Wrapf(ErrCommitSignInvalid, "unknown scheme %#02x", byte(scheme))
	}

	body := c.Sign[2:]
	switch c.Sign[0] {
	case envelopeFull:
		if len(body) != pkLen+sigLen {
			return 0, nil, nil, errors.Wrapf(ErrCommitSignInvalid, "full envelope is %d bytes", len(c.Sign))
		}
		return scheme, body[:pkLen], body[pkLen:], nil

	case envelopeRecover:
		if !HasRecovery(scheme) || len(body) != sigLen {
			return 0, nil, nil, errors.Wrapf(ErrCommitSignInvalid, "recover envelope is %d bytes", len(c.Sign))
		}
		pubKey = RecoverPubKey(scheme, c.SigningHash(), body)
		if pubKey == nil {
			return 0, nil, nil, errors.Wrap(ErrCommitSignInvalid, "public key recovery failed")
		}
		return scheme, pubKey, body, nil

	case envelopeCompact:
		if len(body) != AddressSize+sigLen {
			return 0, nil, nil, errors.Wrapf(ErrCommitSignInvalid, "compact envelope is %d bytes", len(c.Sign))
		}
		if lookup == nil {
			return 0, nil, nil, errors.Wrap(ErrCommitSignInvalid, "compact envelope without a key registry")
		}

		from := Address{}
		copy(from[:], body[:AddressSize])

		entry := lookup(from)
		if len(entry) != 1+pkLen || SigScheme(entry[0]) != scheme {
			return 0, nil, nil, errors.Wrapf(ErrCommitSignInvalid, "no registered %#02x key for %s", byte(scheme), from)
		}
		pubKey = entry[1:]
		if !AddressOfPubKey(scheme, pubKey).Equals(from) {
			return 0, nil, nil, errors.Wrapf(ErrCommitSignInvalid, "registry mismatch for %s", from)
		}

		return scheme, pubKey, body[AddressSize:], nil

	default:
		return 0, nil, nil, ErrCommitUnsigned
	}
}

// From derives the committer address from the Sign envelope, exactly like
// FullTx.From: from the embedded public key on a full envelope, the explicit
// address on a compact one, or the recovered key on a recover one.
func (c *Commitment) From() (Address, error) {
	if len(c.Sign) < 2 {
		return Address{}, ErrCommitUnsigned
	}

	scheme := SigScheme(c.Sign[1])
	pkLen, sigLen := PubKeySize(scheme), SigSize(scheme)
	body := c.Sign[2:]

	switch c.Sign[0] {
	case envelopeFull:
		if pkLen == 0 || len(body) != pkLen+sigLen {
			return Address{}, ErrCommitUnsigned
		}
		return AddressOfPubKey(scheme, body[:pkLen]), nil

	case envelopeCompact:
		if pkLen == 0 || len(body) != AddressSize+sigLen {
			return Address{}, ErrCommitUnsigned
		}
		from := Address{}
		copy(from[:], body[:AddressSize])
		return from, nil

	case envelopeRecover:
		if !HasRecovery(scheme) || len(body) != sigLen {
			return Address{}, ErrCommitUnsigned
		}
		pubKey := RecoverPubKey(scheme, c.SigningHash(), body)
		if pubKey == nil {
			return Address{}, ErrCommitUnsigned
		}
		return AddressOfPubKey(scheme, pubKey), nil

	default:
		return Address{}, ErrCommitUnsigned
	}
}

// Sign signs the commitment with the smallest self-contained envelope,
// mirroring FullTx.Signature: the recover form for recovery schemes, the
// full form (which also registers the key on chain) otherwise.
func (c *Commitment) Signature(privateKey *PrivateKey) error {
	c.Sign = nil
	hash := c.SigningHash()

	sig, err := privateKey.SignHash(hash)
	if err != nil {
		return err
	}

	if HasRecovery(privateKey.Scheme) {
		envelope := make([]byte, 0, 2+len(sig))
		envelope = append(envelope, envelopeRecover, byte(privateKey.Scheme))
		envelope = append(envelope, sig...)
		c.Sign = envelope
		return nil
	}

	pub := privateKey.PublicBytes()
	envelope := make([]byte, 0, 2+len(pub)+len(sig))
	envelope = append(envelope, envelopeFull, byte(privateKey.Scheme))
	envelope = append(envelope, pub...)
	envelope = append(envelope, sig...)

	c.Sign = envelope
	return nil
}

// Verify checks the commitment's signature envelope over its unsigned hash;
// lookup resolves compact envelopes against the on-chain key registry.
func (c *Commitment) Verify(lookup PubKeyResolver) error {
	if len(c.Sign) == 0 {
		return ErrCommitUnsigned
	}

	scheme, pubKey, sig, err := c.envelope(lookup)
	if err != nil {
		return err
	}

	if !VerifyHashSig(scheme, pubKey, c.SigningHash(), sig) {
		return ErrCommitSignInvalid
	}

	return nil
}

// CheckError does a context-free self check: the 32-byte Hash, non-nil Fee,
// matching Height, and a verifying signature envelope.
func (c *Commitment) CheckError(blockHeight uint64, lookup PubKeyResolver) error {
	if len(c.Hash) != HashSize {
		return ErrCommitHashInvalid
	}
	if c.Fee == nil {
		return ErrCommitFeeInvalid
	}
	if c.Height != blockHeight {
		return errors.Wrapf(ErrCommitHeightInvalid, "commitment@%d in block@%d", c.Height, blockHeight)
	}

	return c.Verify(lookup)
}

// CalculateHash feeds the merkle trie: the commitment's unsigned hash.
func (c *Commitment) CalculateHash() ([]byte, error) {
	return c.GetUnsignedHash(), nil
}

// Equals mainly for calculating the trie root over the block's contents.
func (c *Commitment) Equals(other merkletree.Content) (bool, error) {
	oc, ok := other.(*Commitment)
	if !ok {
		return false, nil
	}

	return bytes.Equal(c.GetUnsignedHash(), oc.GetUnsignedHash()), nil
}
