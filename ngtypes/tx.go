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
	// contract slot, like a git commit onto the sender's namespace. The
	// FIRST commit (against the empty base) creates the slot — that is
	// the namespace purchase, burning DeployFee on top of the tx fee
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

// Sender derives the sender address from the public key embedded in
// the signature envelope: whoever holds the key IS the address. The
// state layer enforces every spending rule against this derived sender
func (x *FullTx) Sender() (Address, error) {
	if len(x.Sign) != PublicKeySize+TxSignatureSize {
		return Address{}, ErrTxUnsigned
	}

	return AddressOfPubKey(x.Sign[:PublicKeySize]), nil
}

// Verify checks the tx signature — a flat envelope of the sender's
// public key followed by its signature over the unsigned tx hash. The
// public key is only revealed here, at spend time, never by the
// address itself
func (x *FullTx) Verify() error {
	if x.Height == 0 {
		return nil // ignore all tx error on genesis block
	}

	if len(x.Sign) == 0 {
		return ErrTxUnsigned
	}

	if len(x.Extra) > TxMaxExtraSize {
		return ErrTxExtraExcess
	}

	if len(x.Sign) != PublicKeySize+TxSignatureSize {
		return errors.Wrapf(ErrTxSignInvalid, "signature envelope is %d bytes", len(x.Sign))
	}

	if !VerifyHashSig(x.Sign[:PublicKeySize], x.GetUnsignedHash(), x.Sign[PublicKeySize:]) {
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

// GetHash mainly for calculating the tire root of txs and sign tx.
// The returned hash is sha3_256(tx_with_sign)
func (x *FullTx) GetHash() []byte {
	hash, err := x.CalculateHash()
	if err != nil {
		panic(err)
	}

	return hash
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
	return utils.KeccakSum256(raw)
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *FullTx) CalculateHash() ([]byte, error) {
	raw, err := rlp.EncodeToBytes(x)
	if err != nil {
		return nil, err
	}

	return utils.KeccakSum256(raw), nil
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

	if !bytes.Equal(x.Sign, tx.Sign) {
		return false, nil
	}

	if !bytes.Equal(x.Extra, tx.Extra) {
		return false, nil
	}

	return true, nil
}

// CheckGenerate does a self check for generate tx
func (x *FullTx) CheckGenerate(blockHeight uint64) error {
	if x == nil {
		return ErrBlockNoHeader
	}

	if !(x.TotalExpenditure().Cmp(GetBlockReward(blockHeight)) == 0) {
		return errors.Wrapf(ErrRewardInvalid, "expect %s but reward is %s", GetBlockReward(blockHeight), x.TotalExpenditure())
	}

	if x.Fee.Cmp(big.NewInt(0)) != 0 {
		return errors.Wrap(ErrTxFeeInvalid, "generate's fee should be ZERO")
	}

	if err := x.Verify(); err != nil {
		return err
	}

	// the reward must go to the miner who signed the block's generate
	if x.Height != 0 {
		sender, err := x.Sender()
		if err != nil {
			return err
		}
		if !sender.Equals(x.To) {
			return errors.Wrap(ErrTxRecipientInvalid, "generate must pay its own signer")
		}
	}

	return nil
}

// CheckDestroy does a self check for destroy tx: the sender clears its
// own contract slot
func (x *FullTx) CheckDestroy() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("destroy"); err != nil {
		return err
	}

	return x.Verify()
}

// checkNoTransfer refuses a recipient or value on tx types which only
// act on the sender's own slot
func (x *FullTx) checkNoTransfer(verb string) error {
	if x.To != (Address{}) {
		return errors.Wrapf(ErrTxRecipientInvalid, "%s should have NO recipient", verb)
	}

	if x.Value.Sign() != 0 {
		return errors.Wrapf(ErrTxValueInvalid, "%s should have NO value", verb)
	}

	return nil
}

// CheckTransaction does a self check for normal transaction tx
func (x *FullTx) CheckTransaction() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Value.Sign() < 0 {
		return errors.Wrap(ErrTxValueInvalid, "transact value cannot be negative")
	}

	return x.Verify()
}

// CheckCommit does a self check for commit tx: the sender patches its own
// contract slot
func (x *FullTx) CheckCommit() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("commit"); err != nil {
		return err
	}

	return x.Verify()
}

// CheckActivate does a self check for activate tx
func (x *FullTx) CheckActivate() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("activate"); err != nil {
		return err
	}

	return x.Verify()
}

// CheckDeactivate does a self check for unactivate tx
func (x *FullTx) CheckDeactivate() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if err := x.checkNoTransfer("deactivate"); err != nil {
		return err
	}

	return x.Verify()
}

// Signature signs the tx, embedding the public key and its signature
// as the flat envelope pubkey || sig
func (x *FullTx) Signature(privateKey *PrivateKey) error {
	x.Sign = nil
	hash := x.GetUnsignedHash()

	sig, err := privateKey.SignHash(hash)
	if err != nil {
		return err
	}

	envelope := make([]byte, 0, PublicKeySize+TxSignatureSize)
	envelope = append(envelope, privateKey.PublicBytes()...)
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
