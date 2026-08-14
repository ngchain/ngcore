package ngtypes

import (
	"bytes"
	"encoding/hex"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/c0mm4nd/rlp"
	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/utils"
)

type TxType uint8

const (
	InvalidTx TxType = iota
	GenerateTx
	RegisterTx
	DestroyTx // renamed from logout

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
	Convener     AccountNum
	Participants []Address
	Fee          *big.Int
	Values       []*big.Int // each value is a free-length slice

	Extra []byte
	Sign  []byte `rlp:"optional"`
}

// NewTx is the default constructor for ngtypes.Tx
func NewTx(network Network, txType TxType, height uint64, convener AccountNum, participants []Address, values []*big.Int, fee *big.Int,
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
		Convener:     convener,
		Participants: participants,
		Fee:          fee,
		Values:       values,

		Extra: extraData,
		Sign:  sign,
	}

	return tx
}

// NewUnsignedTx will return an unsigned tx, must using Signature().
func NewUnsignedTx(network Network, txType TxType, height uint64, convener AccountNum, participants []Address, values []*big.Int, fee *big.Int,
	extraData []byte) *FullTx {

	return NewTx(network, txType, height, convener, participants, values, fee, extraData, nil)
}

// IsSigned will return whether the op has been signed.
func (x *FullTx) IsSigned() bool {
	return x.Sign != nil
}

// Verify helps verify the transaction whether signed by the public key
// owner. The signature scheme is BIP-340 schnorr, verified against the
// x-only form of the key (the address keeps the full compressed point)
func (x *FullTx) Verify(publicKey *btcec.PublicKey) error {
	if x.Height == 0 {
		return nil // ignore all tx error on genesis block
	}

	if x.Sign == nil {
		return ErrTxUnsigned
	}

	if publicKey == nil {
		return ErrInvalidPublicKey
	}

	if len(x.Extra) > TxMaxExtraSize {
		return ErrTxExtraExcess
	}

	signature, err := schnorr.ParseSignature(x.Sign)
	if err != nil {
		return errors.Wrap(ErrTxSignInvalid, err.Error())
	}

	if !signature.Verify(x.GetUnsignedHash(), publicKey) {
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
	hash := sha3.Sum256(raw)

	return hash[:]
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *FullTx) CalculateHash() ([]byte, error) {
	raw, err := rlp.EncodeToBytes(x)
	if err != nil {
		return nil, err
	}

	hash := sha3.Sum256(raw)

	return hash[:], nil
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

	if x.Convener != tx.Convener {
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

	if x.Convener != 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "generate's convener should be 0")
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

	publicKey := x.Participants[0].PubKey()
	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckRegister does a self check for register tx
func (x *FullTx) CheckRegister() error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener != 0o1 {
		return errors.Wrap(ErrTxConvenerInvalid, "register's convener should be 1")
	}

	if len(x.Participants) != 1 {
		return errors.Wrap(ErrTxParticipantsInvalid, "register should have only one participant")
	}

	if len(x.Values) != 1 {
		return errors.Wrap(ErrTxValuesInvalid, "register should have only one value")
	}

	if x.Values[0].Cmp(big.NewInt(0)) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "register should have only one value, the amount of which is 0")
	}

	if x.Fee.Cmp(RegisterFee) < 0 {
		return errors.Wrap(ErrTxFeeInvalid, "register should have at least 10NG(one block reward) fee")
	}

	if len(x.Extra) != 1<<3 {
		return errors.Wrap(ErrTxExtraInvalid, "register should have uint64 little-endian bytes as extra")
	}

	publicKey := x.Participants[0].PubKey()
	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckDestroy does a self check for destroy tx
func (x *FullTx) CheckDestroy(publicKey *btcec.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "destroy should have NO participant")
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "destroy's convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "destroy should have no participants")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "destroy should have NO value")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	// RULE: destroy should takes owner's pubKey in Extra for verify and recording to make Tx reversible
	publicKeyFromExtra := utils.Bytes2PublicKey(x.Extra)
	if !publicKey.IsEqual(publicKeyFromExtra) {
		return errors.Wrap(ErrTxExtraInvalid, "invalid raw bytes public key in destroy's Extra field")
	}

	return nil
}

// CheckTransaction does a self check for normal transaction tx
func (x *FullTx) CheckTransaction(publicKey *btcec.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "transact's convener should NOT be 0")
	}

	if len(x.Values) != len(x.Participants) {
		return errors.Wrap(ErrTxParticipantsInvalid, "transact should have same len with participants")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckEdit does a self check for edit tx
func (x *FullTx) CheckEdit(publicKey *btcec.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "edit's convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "edit should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "edit should have NO value")
	}

	return x.Verify(publicKey)
}

// Signature will re-sign the Tx with private key.
// CheckLock does a self check for lock tx
func (x *FullTx) CheckLock(publicKey *btcec.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "lock's convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "lock should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "lock should have NO value")
	}

	return x.Verify(publicKey)
}

// CheckUnlock does a self check for unlock tx
func (x *FullTx) CheckUnlock(publicKey *btcec.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "unlock's convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "unlock should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "unlock should have NO value")
	}

	return x.Verify(publicKey)
}

// Signature signs the tx with a standard BIP-340 schnorr signature.
// Multiple keys are one owner's shards: signing uses the scalar sum,
// whose public key equals the sum of the shard pubkeys — exactly the
// multi-key address from NewAddressFromMultiKeys
func (x *FullTx) Signature(privateKeys ...*btcec.PrivateKey) error {
	if len(privateKeys) == 0 {
		return ErrTxUnsigned
	}

	key, err := CombinePrivateKeys(privateKeys...)
	if err != nil {
		return err
	}

	sign, err := schnorr.Sign(key, x.GetUnsignedHash())
	if err != nil {
		return err
	}

	x.Sign = sign.Serialize()
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
