package ngtypes

import (
	"bytes"
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"
	"github.com/ngchain/go-schnorr"
	"github.com/ngchain/secp256k1"
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

	AppendTx // add content to the tail of contract
	DeleteTx

	LockTx   // TODO: cannot assign nor append, but can run vm
	UnlockTx // TODO: disable vm, but enable assign and append
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

// Verify helps verify the transaction whether signed by the public key owner.
func (x *FullTx) Verify(publicKey secp256k1.PublicKey) error {
	if x.Sign == nil {
		return ErrTxUnsigned
	}

	if publicKey.X == nil || publicKey.Y == nil {
		return ErrInvalidPublicKey
	}

	hash := [32]byte{}
	copy(hash[:], x.GetUnsignedHash())

	var signature [64]byte
	copy(signature[:], x.Sign)

	if len(x.Extra) > TxMaxExtraSize {
		return ErrTxExtraExcess
	}

	var key [33]byte
	copy(key[:], publicKey.SerializeCompressed())
	if ok, err := schnorr.Verify(key, hash, signature); !ok {
		if err != nil {
			return err
		}

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
		if !bytes.Equal(x.Participants[i], tx.Participants[i]) {
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
func (x *FullTx) CheckDestroy(publicKey secp256k1.PublicKey) error {
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
	if !publicKey.IsEqual(&publicKeyFromExtra) {
		return errors.Wrap(ErrTxExtraInvalid, "invalid raw bytes public key in destroy's Extra field")
	}

	return nil
}

// CheckTransaction does a self check for normal transaction tx
func (x *FullTx) CheckTransaction(publicKey secp256k1.PublicKey) error {
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

// CheckAppend does a self check for append tx
func (x *FullTx) CheckAppend(key secp256k1.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "append's convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "append should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "append should have NO value")
	}

	err := x.Verify(key)
	if err != nil {
		return err
	}

	// check this on chain
	// var appendExtra AppendExtra
	// err = rlp.DecodeBytes(x.Extra, &appendExtra)
	// if err != nil {
	//	return err
	// }

	return nil
}

// CheckDelete does a self check for delete tx
func (x *FullTx) CheckDelete(publicKey secp256k1.PublicKey) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "deleteTx convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return errors.Wrap(ErrTxParticipantsInvalid, "deleteTx should have NO participant")
	}

	if len(x.Values) != 0 {
		return errors.Wrap(ErrTxValuesInvalid, "deleteTx should have NO value")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// Signature will re-sign the Tx with private key.
func (x *FullTx) Signature(privateKeys ...*secp256k1.PrivateKey) (err error) {
	ds := make([]*big.Int, len(privateKeys))
	for i := range privateKeys {
		ds[i] = privateKeys[i].D
	}

	hash := [32]byte{}
	copy(hash[:], x.GetUnsignedHash())

	sign, err := schnorr.AggregateSignatures(ds, hash)
	if err != nil {
		panic(err)
	}

	x.Sign = sign[:]
	return
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
