package ngtypes

import (
	"bytes"
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

type TxType uint8

const (
	InvalidTx TxType = iota
	GenerateTx
	DestroyTx

	TransactTx

	// CommitTx commits a change set (diff hunks) onto the sender's
	// contract slot, like a git commit onto the sender's namespace.
	// The FIRST commit (against the empty base) creates the slot
	CommitTx

	ActivateTx   // freeze the contract: no more commits, and the vm gets active
	DeactivateTx // disable the vm, and enable committing again
)

// FullTx is the basic implement of Tx (transaction, or operation)
type FullTx struct {
	Network Network
	Type    TxType
	Height  uint64 // lock the tx on the specific height, rather than the hash, to make the tx can act on forking
	To      Address
	Value   *big.Int
	Fee     *big.Int

	Extra []byte
	Sign  []byte `rlp:"optional"`
}

// NewTx is the default constructor for ngtypes.Tx
func NewTx(network Network, txType TxType, height uint64, to Address, value, fee *big.Int,
	extraData, sign []byte) *FullTx {
	if value == nil {
		value = big.NewInt(0)
	}

	if fee == nil {
		fee = big.NewInt(0)
	}

	if extraData == nil {
		extraData = []byte{}
	}

	if sign == nil {
		sign = []byte{}
	}

	tx := &FullTx{
		Network: network,
		Type:    txType,
		Height:  height,
		To:      to,
		Value:   value,
		Fee:     fee,

		Extra: extraData,
		Sign:  sign,
	}

	return tx
}

// NewUnsignedTx will return an unsigned tx, must using Signature().
func NewUnsignedTx(network Network, txType TxType, height uint64, to Address, value, fee *big.Int,
	extraData []byte) *FullTx {

	return NewTx(network, txType, height, to, value, fee, extraData, nil)
}

// IsSigned will return whether the op has been signed.
func (x *FullTx) IsSigned() bool {
	return len(x.Sign) != 0
}

// The signature envelope has three forms, told apart by a leading
// form tag; all carry the scheme byte so sizes parse unambiguously:
//
//	full:    0x01 ‖ scheme ‖ pubkey ‖ sig — reveals (and registers) the key
//	compact: 0x02 ‖ scheme ‖ From(32) ‖ sig — omits the key; valid only
//	         once a PRIOR block registered the address's key on chain
//	recover: 0x03 ‖ scheme ‖ sig — for schemes with public key
//	         RECOVERY (secp256k1): the key AND the sender fall out of
//	         the signature itself, eth-style; 67 bytes all included
//
// A PubKeyResolver looks a registered key (scheme ‖ pubkey) up by
// address; nil means no registry is available and only full/recover
// envelopes verify.
type PubKeyResolver func(Address) []byte

const (
	envelopeFull    byte = 0x01
	envelopeCompact byte = 0x02
	envelopeRecover byte = 0x03
)

// IsCompactEnvelope reports whether the tx uses the compact form
func (x *FullTx) IsCompactEnvelope() bool {
	return len(x.Sign) > 0 && x.Sign[0] == envelopeCompact
}

// EnvelopeScheme returns the signature scheme the envelope declares
func (x *FullTx) EnvelopeScheme() SigScheme {
	if len(x.Sign) < 2 {
		return 0
	}

	return SigScheme(x.Sign[1])
}

// envelope splits Sign into the scheme, public key (resolving the
// compact form through the registry) and the signature
func (x *FullTx) envelope(lookup PubKeyResolver) (scheme SigScheme, pubKey, sig []byte, err error) {
	if len(x.Sign) < 2 {
		return 0, nil, nil, ErrTxUnsigned
	}

	scheme = SigScheme(x.Sign[1])
	pkLen, sigLen := PubKeySize(scheme), SigSize(scheme)
	if pkLen == 0 {
		return 0, nil, nil, errors.Wrapf(ErrTxSignInvalid, "unknown scheme %#02x", byte(scheme))
	}

	body := x.Sign[2:]
	switch x.Sign[0] {
	case envelopeFull:
		if len(body) != pkLen+sigLen {
			return 0, nil, nil, errors.Wrapf(ErrTxSignInvalid, "full envelope is %d bytes", len(x.Sign))
		}
		return scheme, body[:pkLen], body[pkLen:], nil

	case envelopeRecover:
		if !HasRecovery(scheme) || len(body) != sigLen {
			return 0, nil, nil, errors.Wrapf(ErrTxSignInvalid, "recover envelope is %d bytes", len(x.Sign))
		}
		pubKey = RecoverPubKey(scheme, x.GetUnsignedHash(), body)
		if pubKey == nil {
			return 0, nil, nil, errors.Wrap(ErrTxSignInvalid, "public key recovery failed")
		}
		return scheme, pubKey, body, nil

	case envelopeCompact:
		if len(body) != AddressSize+sigLen {
			return 0, nil, nil, errors.Wrapf(ErrTxSignInvalid, "compact envelope is %d bytes", len(x.Sign))
		}
		if lookup == nil {
			return 0, nil, nil, errors.Wrap(ErrTxSignInvalid, "compact envelope without a key registry")
		}

		from := Address{}
		copy(from[:], body[:AddressSize])

		entry := lookup(from)
		if len(entry) != 1+pkLen || SigScheme(entry[0]) != scheme {
			return 0, nil, nil, errors.Wrapf(ErrTxSignInvalid, "no registered %#02x key for %s", byte(scheme), from)
		}
		pubKey = entry[1:]
		if !AddressOfPubKey(scheme, pubKey).Equals(from) {
			return 0, nil, nil, errors.Wrapf(ErrTxSignInvalid, "registry mismatch for %s", from)
		}

		return scheme, pubKey, body[AddressSize:], nil

	default:
		return 0, nil, nil, ErrTxUnsigned
	}
}

// From derives the From address: from the embedded public key on a
// full envelope, or the explicit address on a compact one. Whoever
// holds the key IS the address; the state layer enforces every
// spending rule against this derived From address
func (x *FullTx) From() (Address, error) {
	if len(x.Sign) < 2 {
		return Address{}, ErrTxUnsigned
	}

	scheme := SigScheme(x.Sign[1])
	pkLen, sigLen := PubKeySize(scheme), SigSize(scheme)
	body := x.Sign[2:]

	switch x.Sign[0] {
	case envelopeFull:
		if pkLen == 0 || len(body) != pkLen+sigLen {
			return Address{}, ErrTxUnsigned
		}
		return AddressOfPubKey(scheme, body[:pkLen]), nil

	case envelopeCompact:
		if pkLen == 0 || len(body) != AddressSize+sigLen {
			return Address{}, ErrTxUnsigned
		}
		from := Address{}
		copy(from[:], body[:AddressSize])
		return from, nil

	case envelopeRecover:
		if !HasRecovery(scheme) || len(body) != sigLen {
			return Address{}, ErrTxUnsigned
		}
		pubKey := RecoverPubKey(scheme, x.GetUnsignedHash(), body)
		if pubKey == nil {
			return Address{}, ErrTxUnsigned
		}
		return AddressOfPubKey(scheme, pubKey), nil

	default:
		return Address{}, ErrTxUnsigned
	}
}

// Verify checks the tx signature envelope over the unsigned tx hash.
// lookup resolves compact envelopes against the on-chain key registry
// (nil accepts full envelopes only)
func (x *FullTx) Verify(lookup PubKeyResolver) error {
	if x.Height == 0 {
		return nil // ignore all tx error on genesis block
	}

	if len(x.Sign) == 0 {
		return ErrTxUnsigned
	}

	if len(x.Extra) > TxMaxExtraSize {
		return ErrTxExtraExcess
	}

	scheme, pubKey, sig, err := x.envelope(lookup)
	if err != nil {
		return err
	}

	if !VerifyHashSig(scheme, pubKey, x.GetUnsignedHash(), sig) {
		return ErrTxSignInvalid
	}

	return nil
}

// BS58 is a tx's Readable Raw in string.
func (x *FullTx) BS58() string {
	b, err := rlp.EncodeToBytes(x)
	if err != nil {
		log.Error(err)
	}

	return base58.FastBase58Encoding(b)
}

// ID is a tx's Readable ID(hash) in string.
func (x *FullTx) ID() string {
	return hex.EncodeToString(x.GetHash())
}

// GetHash returns the txid: the hash of the tx WITHOUT its signature
// envelope. Witness data is committed separately in the block header,
// so pruning or aggregating signatures later never disturbs the txid
// nor any history commitment built on it
func (x *FullTx) GetHash() []byte {
	return x.GetUnsignedHash()
}

// GetUnsignedHash mainly for signing and verifying.
// The returned hash is sha3_256(tx_without_sign)
func (x *FullTx) GetUnsignedHash() []byte {
	sign := x.Sign
	x.Sign = nil
	raw, err := rlp.EncodeToBytes(x)
	if err != nil {
		panic(err)
	}

	x.Sign = sign
	return utils.Hash256(raw)
}

// CalculateHash feeds the merkle trie: the txid (unsigned hash)
func (x *FullTx) CalculateHash() ([]byte, error) {
	return x.GetUnsignedHash(), nil
}

// Equals mainly for calculating the tire root of txs.
func (x *FullTx) Equals(other merkletree.Content) (bool, error) {
	tx, ok := other.(*FullTx)
	if !ok {
		panic("comparing with non-tx struct")
	}

	if x.Network != tx.Network {
		return false, nil
	}

	if x.Height != tx.Height {
		return false, nil
	}

	if x.To != tx.To {
		return false, nil
	}

	if x.Value.Cmp(tx.Value) != 0 {
		return false, nil
	}

	if x.Fee.Cmp(tx.Fee) != 0 {
		return false, nil
	}

	if !bytes.Equal(x.Extra, tx.Extra) {
		return false, nil
	}

	return true, nil
}

// CheckGenerate does a self check for generate tx
func (x *FullTx) CheckGenerate(blockHeight uint64, lookup PubKeyResolver) error {
	if x == nil {
		return ErrBlockNoHeader
	}

	if !(x.TotalExpenditure().Cmp(GetBlockReward(blockHeight)) == 0) {
		return errors.Wrapf(ErrRewardInvalid, "expect %s but reward is %s", GetBlockReward(blockHeight), x.TotalExpenditure())
	}

	if x.Fee.Cmp(big.NewInt(0)) != 0 {
		return errors.Wrap(ErrTxFeeInvalid, "generate's fee should be ZERO")
	}

	if err := x.Verify(lookup); err != nil {
		return err
	}

	// the reward must go to the miner who signed the block's generate
	if x.Height != 0 {
		from, err := x.From()
		if err != nil {
			return err
		}
		if !from.Equals(x.To) {
			return errors.Wrap(ErrTxToInvalid, "generate must pay its own signer")
		}
	}

	return nil
}

// CheckDestroy does a self check for destroy tx: the From address clears its
// own contract slot
func (x *FullTx) CheckDestroy(lookup PubKeyResolver) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("destroy"); err != nil {
		return err
	}

	return x.Verify(lookup)
}

// checkNoTransfer refuses a To address or value on tx types which
// only act on the From address's own slot
func (x *FullTx) checkNoTransfer(verb string) error {
	if x.To != (Address{}) {
		return errors.Wrapf(ErrTxToInvalid, "%s must not set To", verb)
	}

	if x.Value.Sign() != 0 {
		return errors.Wrapf(ErrTxValueInvalid, "%s should have NO value", verb)
	}

	return nil
}

// CheckTransaction does a self check for normal transaction tx
func (x *FullTx) CheckTransaction(lookup PubKeyResolver) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Value.Sign() < 0 {
		return errors.Wrap(ErrTxValueInvalid, "transact value cannot be negative")
	}

	return x.Verify(lookup)
}

// CheckCommit does a self check for commit tx: the From address patches its own
// contract slot
func (x *FullTx) CheckCommit(lookup PubKeyResolver) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("commit"); err != nil {
		return err
	}

	return x.Verify(lookup)
}

// CheckActivate does a self check for activate tx
func (x *FullTx) CheckActivate(lookup PubKeyResolver) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("activate"); err != nil {
		return err
	}

	return x.Verify(lookup)
}

// CheckDeactivate does a self check for unactivate tx
func (x *FullTx) CheckDeactivate(lookup PubKeyResolver) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("deactivate"); err != nil {
		return err
	}

	return x.Verify(lookup)
}

// Signature signs the tx with the smallest self-contained envelope:
// the 67-byte recover form for schemes with key recovery, the FULL
// form (which also registers the key on chain) otherwise
func (x *FullTx) Signature(privateKey *PrivateKey) error {
	x.Sign = nil
	hash := x.GetUnsignedHash()

	sig, err := privateKey.SignHash(hash)
	if err != nil {
		return err
	}

	if HasRecovery(privateKey.Scheme) {
		envelope := make([]byte, 0, 2+len(sig))
		envelope = append(envelope, envelopeRecover, byte(privateKey.Scheme))
		envelope = append(envelope, sig...)
		x.Sign = envelope
		return nil
	}

	pub := privateKey.PublicBytes()
	envelope := make([]byte, 0, 2+len(pub)+len(sig))
	envelope = append(envelope, envelopeFull, byte(privateKey.Scheme))
	envelope = append(envelope, pub...)
	envelope = append(envelope, sig...)

	x.Sign = envelope
	return nil
}

// SignatureCompact signs the tx with the COMPACT envelope, saving the
// public key bytes; it only validates once the address's key is
// registered on chain by an earlier full-envelope tx. Recovery
// schemes just use their (even smaller) recover envelope
func (x *FullTx) SignatureCompact(privateKey *PrivateKey) error {
	if HasRecovery(privateKey.Scheme) {
		return x.Signature(privateKey)
	}

	x.Sign = nil
	hash := x.GetUnsignedHash()

	sig, err := privateKey.SignHash(hash)
	if err != nil {
		return err
	}

	from := AddressOfPubKey(privateKey.Scheme, privateKey.PublicBytes())
	envelope := make([]byte, 0, 2+AddressSize+len(sig))
	envelope = append(envelope, envelopeCompact, byte(privateKey.Scheme))
	envelope = append(envelope, from[:]...)
	envelope = append(envelope, sig...)

	x.Sign = envelope
	return nil
}

func (x *FullTx) ManuallySetSignature(sign []byte) {
	x.Sign = sign
}

// TotalExpenditure helps calculate the total expenditure which the tx caller should pay
func (x *FullTx) TotalExpenditure() *big.Int {
	return new(big.Int).Add(x.Fee, x.Value)
}
