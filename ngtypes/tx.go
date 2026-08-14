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
	return len(x.Sign) != 0
}

// TxKeySet is the signature envelope carried in FullTx.Sign: the whole
// keyset descriptor (which must hash to the owner address) plus exactly
// threshold member signatures. Public keys are only revealed here, at
// spend time — never by the address itself
type TxKeySet struct {
	Threshold uint64
	PubKeys   [][]byte
	Sigs      [][]byte // parallel to PubKeys; empty slot = did not sign
}

// Verify checks the tx signature envelope against the owner address:
// the embedded keyset must hash to the address, and exactly threshold
// member signatures must all be valid over the unsigned tx hash
func (x *FullTx) Verify(owner Address) error {
	if x.Height == 0 {
		return nil // ignore all tx error on genesis block
	}

	if len(x.Sign) == 0 {
		return ErrTxUnsigned
	}

	if len(x.Extra) > TxMaxExtraSize {
		return ErrTxExtraExcess
	}

	var keyset TxKeySet
	if err := rlp.DecodeBytes(x.Sign, &keyset); err != nil {
		return errors.Wrap(ErrTxSignInvalid, err.Error())
	}

	if len(keyset.Sigs) != len(keyset.PubKeys) {
		return errors.Wrap(ErrTxSignInvalid, "keyset arrays misaligned")
	}
	if keyset.Threshold > MaxKeysetKeys {
		return errors.Wrapf(ErrTxSignInvalid, "threshold %d", keyset.Threshold)
	}

	addr, err := KeysetAddress(int(keyset.Threshold), keyset.PubKeys)
	if err != nil {
		return errors.Wrap(ErrTxSignInvalid, err.Error())
	}
	if !addr.Equals(owner) {
		return errors.Wrap(ErrTxSignInvalid, "keyset does not hash to the owner address")
	}

	hash := x.GetUnsignedHash()
	signed := 0
	for i := range keyset.Sigs {
		if len(keyset.Sigs[i]) == 0 {
			continue
		}
		if !VerifyHashSig(keyset.PubKeys[i], hash, keyset.Sigs[i]) {
			return errors.Wrapf(ErrTxSignInvalid, "member %d signature invalid", i)
		}
		signed++
	}

	// EXACTLY threshold signatures: with surplus valid signatures a
	// third party could strip one and mint a second valid encoding of
	// the same tx (signature malleability)
	if signed != int(keyset.Threshold) {
		return errors.Wrapf(ErrTxSignInvalid, "%d signatures, threshold is %d", signed, keyset.Threshold)
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

	if err := x.Verify(x.Participants[0]); err != nil {
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

	if err := x.Verify(x.Participants[0]); err != nil {
		return err
	}

	return nil
}

// CheckDestroy does a self check for destroy tx
func (x *FullTx) CheckDestroy(owner Address) error {
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

	// the signature envelope itself records the full keyset, so the
	// old rule of echoing the public key into Extra is obsolete
	return x.Verify(owner)
}

// CheckTransaction does a self check for normal transaction tx
func (x *FullTx) CheckTransaction(owner Address) error {
	if x == nil {
		return ErrTxNoHeader
	}

	if x.Convener == 0 {
		return errors.Wrap(ErrTxConvenerInvalid, "transact's convener should NOT be 0")
	}

	if len(x.Values) != len(x.Participants) {
		return errors.Wrap(ErrTxParticipantsInvalid, "transact should have same len with participants")
	}

	if err := x.Verify(owner); err != nil {
		return err
	}

	return nil
}

// CheckEdit does a self check for edit tx
func (x *FullTx) CheckEdit(owner Address) error {
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

	return x.Verify(owner)
}

// Signature will re-sign the Tx with private key.
// CheckLock does a self check for lock tx
func (x *FullTx) CheckLock(owner Address) error {
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

	return x.Verify(owner)
}

// CheckUnlock does a self check for unlock tx
func (x *FullTx) CheckUnlock(owner Address) error {
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

	return x.Verify(owner)
}

// Signature signs with the all-must-sign keyset of the given keys
// (N-of-N), matching NewAddress / NewAddressFromMultiKeys
func (x *FullTx) Signature(privateKeys ...*PrivateKey) error {
	members := make([][]byte, len(privateKeys))
	for i := range privateKeys {
		members[i] = privateKeys[i].PublicBytes()
	}

	return x.SignMultisig(len(privateKeys), members, privateKeys...)
}

// SignMultisig signs a threshold-of-N keyset: members lists the WHOLE
// keyset's public keys (as committed by the address), signers the
// exactly-threshold subset whose keys are at hand
func (x *FullTx) SignMultisig(threshold int, members [][]byte, signers ...*PrivateKey) error {
	if len(signers) != threshold {
		return errors.Wrapf(ErrTxSignInvalid, "need exactly %d signers, got %d", threshold, len(signers))
	}

	keyset := TxKeySet{
		Threshold: uint64(threshold),
		PubKeys:   make([][]byte, len(members)),
		Sigs:      make([][]byte, len(members)),
	}
	for i := range members {
		keyset.PubKeys[i] = members[i]
		keyset.Sigs[i] = []byte{}
	}

	x.Sign = nil
	hash := x.GetUnsignedHash()

	for _, signer := range signers {
		slot := -1
		pub := signer.PublicBytes()
		for i := range members {
			if bytes.Equal(members[i], pub) && len(keyset.Sigs[i]) == 0 {
				slot = i
				break
			}
		}
		if slot < 0 {
			return errors.Wrap(ErrTxSignInvalid, "a signer is not an unsigned member of the keyset")
		}

		sig, err := signer.SignHash(hash)
		if err != nil {
			return err
		}
		keyset.Sigs[slot] = sig
	}

	raw, err := rlp.EncodeToBytes(&keyset)
	if err != nil {
		return err
	}

	x.Sign = raw
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
