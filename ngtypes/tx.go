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

	EditTx // apply a patch (a set of hunks) onto the contract text

	LockTx   // freeze the contract: no more editing, and the vm gets active
	UnlockTx // disable the vm, and enable editing again
)

// FullTx is the basic implement of Tx (transaction, or operation)
type FullTx struct {
	Network      Network
	Type         TxType
	Height       uint64 // lock the tx on the specific height, rather than the hash, to make the tx can act on forking
	Participants []Address
	Fee          *big.Int
	Values       []*big.Int // each value is a free-length slice

	Extra []byte
	Sign  []byte `rlp:"optional"`
}

// NewTx is the default constructor for ngtypes.Tx
func NewTx(network Network, txType TxType, height uint64, participants []Address, values []*big.Int, fee *big.Int,
	extraData, sign []byte) *FullTx {
	if participants == nil {
		participants = []Address{}
	}

	if values == nil {
		values = []*big.Int{}
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
		Network:      network,
		Type:         txType,
		Height:       height,
		Participants: participants,
		Fee:          fee,
		Values:       values,

		Extra: extraData,
		Sign:  sign,
	}

	return tx
}

// NewUnsignedTx will return an unsigned tx, must using Signature().
func NewUnsignedTx(network Network, txType TxType, height uint64, participants []Address, values []*big.Int, fee *big.Int,
	extraData []byte) *FullTx {

	return NewTx(network, txType, height, participants, values, fee, extraData, nil)
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

	if len(x.Participants) != len(tx.Participants) {
		return false, nil
	}

	for i := range x.Participants {
		if x.Participants[i] != tx.Participants[i] {
			return false, nil
		}
	}

	if len(x.Values) != len(tx.Values) {
		return false, nil
	}

	for i := range x.Values {
		if x.Values[i].Cmp(tx.Values[i]) != 0 {
			return false, nil
		}
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

	if len(x.Values) != len(x.Participants) {
		return errors.Wrap(ErrTxParticipantsInvalid, "generate should have same len with participants")
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
		if len(x.Participants) != 1 || !sender.Equals(x.Participants[0]) {
			return errors.Wrap(ErrTxParticipantsInvalid, "generate must pay its own signer")
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

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "destroy should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "destroy should have NO value")
	}

	return x.Verify()
}

// CheckTransaction does a self check for normal transaction tx
func (x *FullTx) CheckTransaction() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if len(x.Values) != len(x.Participants) {
		return errors.Wrap(ErrTxParticipantsInvalid, "transact should have same len with participants")
	}

	return x.Verify()
}

// CheckEdit does a self check for edit tx: the sender patches its own
// contract slot
func (x *FullTx) CheckEdit() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "edit should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "edit should have NO value")
	}

	return x.Verify()
}

// CheckLock does a self check for lock tx
func (x *FullTx) CheckLock() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "lock should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "lock should have NO value")
	}

	return x.Verify()
}

// CheckUnlock does a self check for unlock tx
func (x *FullTx) CheckUnlock() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "unlock should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "unlock should have NO value")
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
	total := big.NewInt(0)

	for i := range x.Values {
		total.Add(total, x.Values[i])
	}

	return new(big.Int).Add(x.Fee, total)
}
