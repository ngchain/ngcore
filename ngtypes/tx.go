package ngtypes

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/c0mm4nd/rlp"
	"math/big"

	"github.com/ngchain/go-schnorr"
	"github.com/ngchain/secp256k1"
	"golang.org/x/crypto/sha3"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/utils"
)

const (
	InvalidTx  = 0
	GenerateTx = 1
	RegisterTx = 2
	DestroyTx  = 3 // renamed from logout

	TransactTx = 4

	AppendTx = 5 // add content to the tail of contract
	DeleteTx = 6

	LockTx   = 7 // TODO: cannot assign nor append, but can run vm
	UnlockTx = 8 // TODO: disable vm, but enable assign and append
)

// Errors for Tx
var (
	// ErrTxWrongSign occurs when the signature of the Tx doesnt match the Tx 's caller/account
	ErrTxWrongSign = errors.New("the signer of transaction is not the own of the account")
)

type Tx struct {
	Network      Network
	Type         uint8
	Height       uint64 // lock the tx on the specific height, rather than the hash, to make the tx can act on forking
	Convener     AccountNum
	Participants []Address
	Fee          *big.Int
	Values       []*big.Int // each value is a free-length slice

	Extra []byte
	Sign  []byte
}

// NewTx is the default constructor for ngtypes.Tx
func NewTx(network Network, txType uint8, height uint64, convener AccountNum, participants []Address, values []*big.Int, fee *big.Int,
	extraData, sign, hash []byte) *Tx {
	tx := &Tx{
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
func NewUnsignedTx(network Network, txType uint8, height uint64, convener AccountNum, participants []Address, values []*big.Int, fee *big.Int,
	extraData []byte) *Tx {

	return NewTx(network, txType, height, convener, participants, values, fee, extraData, nil, nil)
}

// IsSigned will return whether the op has been signed.
func (x *Tx) IsSigned() bool {
	return x.Sign != nil
}

// Verify helps verify the transaction whether signed by the public key owner.
func (x *Tx) Verify(publicKey secp256k1.PublicKey) error {
	if x.Sign == nil {
		return fmt.Errorf("unsigned transaction")
	}

	if publicKey.X == nil || publicKey.Y == nil {
		return fmt.Errorf("illegal public key")
	}

	hash := [32]byte{}
	copy(hash[:], x.GetHash())

	var signature [64]byte
	copy(signature[:], x.Sign)

	var key [33]byte
	copy(key[:], publicKey.SerializeCompressed())

	if ok, err := schnorr.Verify(key, hash, signature); !ok {
		if err != nil {
			return err
		}

		return ErrTxWrongSign
	}

	return nil
}

// BS58 is a tx's Readable Raw in string.
func (x *Tx) BS58() string {
	b, err := rlp.EncodeToBytes(x)
	if err != nil {
		log.Error(err)
	}

	return base58.FastBase58Encoding(b)
}

// ID is a tx's Readable ID(hash) in string.
func (x *Tx) ID() string {
	return hex.EncodeToString(x.GetHash())
}

// GetHash mainly for calculating the tire root of txs and sign tx.
func (x *Tx) GetHash() []byte {
	hash, err := x.CalculateHash()
	if err != nil {
		panic(err)
	}

	return hash
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *Tx) CalculateHash() ([]byte, error) {
	raw, err := rlp.EncodeToBytes(x)
	if err != nil {
		return nil, err
	}

	hash := sha3.Sum256(raw)

	return hash[:], nil
}

// Equals mainly for calculating the tire root of txs.
func (x *Tx) Equals(other merkletree.Content) (bool, error) {
	tx, ok := other.(*Tx)
	if !ok {
		return false, errors.New("invalid transaction type")
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
func (x *Tx) CheckGenerate(blockHeight uint64) error {
	if x == nil {
		return errors.New("generate is missing header")
	}

	if x.Convener != 0 {
		return fmt.Errorf("generate's convener should be 0")
	}

	if len(x.Values) != len(x.Participants) {
		return fmt.Errorf("generate should have same len with participants")
	}

	if !(x.TotalExpenditure().Cmp(GetBlockReward(blockHeight)) == 0) {
		return fmt.Errorf("wrong block reward: expect %s but value is %s", GetBlockReward(blockHeight), x.TotalExpenditure())
	}

	if x.Fee.Cmp(big.NewInt(0)) != 0 {
		return fmt.Errorf("generate's fee should be ZERO")
	}

	publicKey := x.Participants[0].PubKey()
	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckRegister does a self check for register tx
func (x *Tx) CheckRegister() error {
	if x == nil {
		return errors.New("register is missing header")
	}

	if x.Convener != 01 {
		return fmt.Errorf("register's convener should be 1")
	}

	if len(x.Participants) != 1 {
		return fmt.Errorf("register should have only one participant")
	}

	if len(x.Values) != 1 {
		return fmt.Errorf("register should have only one value")
	}

	if x.Values[0].Cmp(big.NewInt(0)) != 0 {
		return fmt.Errorf("register should have only one value, the amount of which is 0")
	}

	if x.Fee.Cmp(RegisterFee) < 0 {
		return fmt.Errorf("register should have at least 10NG(one block reward) fee")
	}

	if len(x.Extra) != 1<<3 {
		return fmt.Errorf("register should have uint64 little-endian bytes as extra")
	}

	publicKey := x.Participants[0].PubKey()
	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckLogout does a self check for logout tx
func (x *Tx) CheckLogout(publicKey secp256k1.PublicKey) error {
	if x == nil {
		return errors.New("logout is missing header")
	}

	if len(x.Participants) != 0 {
		return fmt.Errorf("logout should have NO participant")
	}

	if x.Convener == 0 {
		return fmt.Errorf("logout's convener should NOT be 0")
	}

	if len(x.Values) != 0 {
		return fmt.Errorf("logout should have NO value")
	}

	if len(x.Values) != len(x.Participants) {
		return fmt.Errorf("logout should have same len with participants")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	// RULE: logout should takes owner's pubKey in Extra for verify and recording to make Tx reversible
	_publicKey := utils.Bytes2PublicKey(x.Extra)
	if !publicKey.IsEqual(&_publicKey) {
		return fmt.Errorf("invalid raw bytes public key in logout's Extra field")
	}

	return nil
}

// CheckTransaction does a self check for normal transaction tx
func (x *Tx) CheckTransaction(publicKey secp256k1.PublicKey) error {
	if x == nil {
		return errors.New("transaction is missing header")
	}

	if x.Convener == 0 {
		return fmt.Errorf("transaction's convener should NOT be 0")
	}

	if len(x.Values) != len(x.Participants) {
		return fmt.Errorf("transaction should have same len with participants")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckAppend does a self check for append tx
func (x *Tx) CheckAppend(key secp256k1.PublicKey) error {
	if x == nil {
		return errors.New("append is missing header")
	}

	if len(x.Participants) != 0 {
		return fmt.Errorf("append should have NO participant")
	}

	if x.Convener == 0 {
		return fmt.Errorf("append's convener should NOT be 0")
	}

	if len(x.Values) != 0 {
		return fmt.Errorf("append should have NO value")
	}

	err := x.Verify(key)
	if err != nil {
		return err
	}

	// check this on chain
	//var appendExtra AppendExtra
	//err = rlp.DecodeBytes(x.Extra, &appendExtra)
	//if err != nil {
	//	return err
	//}

	return nil
}

// CheckDelete does a self check for delete tx
func (x *Tx) CheckDelete(publicKey secp256k1.PublicKey) error {
	if x == nil {
		return errors.New("deleteTx is missing header")
	}

	if x.Convener == 0 {
		return fmt.Errorf("deleteTx's convener should NOT be 0")
	}

	if len(x.Participants) != 0 {
		return fmt.Errorf("deleteTx should have NO participant")
	}

	if len(x.Values) != 0 {
		return fmt.Errorf("deleteTx should have NO value")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// Signature will re-sign the Tx with private key.
func (x *Tx) Signature(privateKeys ...*secp256k1.PrivateKey) (err error) {
	ds := make([]*big.Int, len(privateKeys))
	for i := range privateKeys {
		ds[i] = privateKeys[i].D
	}

	hash := [32]byte{}
	copy(hash[:], x.GetHash())

	sign, err := schnorr.AggregateSignatures(ds, hash)
	if err != nil {
		panic(err)
	}

	x.Sign = sign[:]

	return
}

func (x *Tx) ManuallySetSignature(sign []byte) {
	x.Sign = sign
}

// TotalExpenditure helps calculate the total expenditure which the tx caller should pay
func (x *Tx) TotalExpenditure() *big.Int {
	total := big.NewInt(0)

	for i := range x.Values {
		total.Add(total, x.Values[i])
	}

	return new(big.Int).Add(x.Fee, total)
}
