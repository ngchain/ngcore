package ngtypes

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ngchain/go-schnorr"

	"github.com/ngchain/secp256k1"
	"golang.org/x/crypto/sha3"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/utils"
)

// Errors for Tx
var (
	ErrTxIsNotSigned = errors.New("the transaction is not signed")
	ErrTxWrongSign   = errors.New("the signer of transaction is not the own of the account")
)

// NewUnsignedTx will return an unsigned tx, must using Signature().
func NewUnsignedTx(txType TxType, convener uint64, participants [][]byte, values []*big.Int, fee *big.Int,
	nonce uint64, extraData []byte) *Tx {
	header := &TxHeader{
		Type:         txType,
		Convener:     convener,
		Participants: participants,
		Fee:          fee.Bytes(),
		Values:       BigIntsToBytesList(values),
		Nonce:        nonce,
		Extra:        extraData,
	}

	return &Tx{
		Network: Network,
		Header:  header,
		Sign:    nil,
	}
}

// IsSigned will return whether the op has been signed.
func (x *Tx) IsSigned() bool {
	return x.Sign != nil
}

// Verify helps verify the transaction whether signed by the public key owner.
func (x *Tx) Verify(publicKey secp256k1.PublicKey) error {
	if x.Network != Network {
		return fmt.Errorf("tx's network id is incorrect")
	}

	if x.Sign == nil {
		return fmt.Errorf("unsigned transaction")
	}

	if publicKey.X == nil || publicKey.Y == nil {
		return fmt.Errorf("illegal public key")
	}

	b, err := utils.Proto.Marshal(x.Header)
	if err != nil {
		return err
	}

	var signature [64]byte

	copy(signature[:], x.Sign)

	var key [33]byte

	copy(key[:], publicKey.SerializeCompressed())

	if ok, err := schnorr.Verify(key, sha3.Sum256(b), signature); !ok {
		if err != nil {
			return err
		}

		return ErrTxWrongSign
	}

	return nil
}

// BS58 is a tx's Readable Raw in string.
func (x *Tx) BS58() string {
	b, err := utils.Proto.Marshal(x)
	if err != nil {
		log.Error(err)
	}

	return base58.FastBase58Encoding(b)
}

// ID is a tx's Readable ID(hash) in string.
func (x *Tx) ID() string {
	hash, err := x.CalculateHash()
	if err != nil {
		log.Error(err)
	}

	return hex.EncodeToString(hash)
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *Tx) CalculateHash() ([]byte, error) {
	raw, err := utils.Proto.Marshal(x)
	if err != nil {
		log.Error(err)
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

	otherRawHeader, err := utils.Proto.Marshal(tx.Header)
	if err != nil {
		return false, err
	}

	otherHash := sha3.Sum256(otherRawHeader)

	selfRawHeader, err := utils.Proto.Marshal(x.Header)
	if err != nil {
		return false, err
	}

	selfHash := sha3.Sum256(selfRawHeader)

	return bytes.Equal(selfHash[:], otherHash[:]), nil
}

// TxsToMerkleTreeContents make a []merkletree.Content whose values is from txs.
func TxsToMerkleTreeContents(txs []*Tx) []merkletree.Content {
	mtc := make([]merkletree.Content, len(txs))
	for i := range txs {
		mtc[i] = txs[i]
	}

	return mtc
}

// BigIntsToBytesList is a helper converts bigInts to raw bytes slice.
func BigIntsToBytesList(bigInts []*big.Int) [][]byte {
	bytesList := make([][]byte, len(bigInts))
	for i := 0; i < len(bigInts); i++ {
		bytesList[i] = bigInts[i].Bytes()
	}

	return bytesList
}

func (x *Tx) CheckGenerate() error {
	if x.Header == nil {
		return errors.New("generate is missing header")
	}

	if x.GetConvener() != 0 {
		return fmt.Errorf("generate's convener should be 0")
	}

	if len(x.GetValues()) != len(x.GetParticipants()) {
		return fmt.Errorf("generate should have same len with participants")
	}

	if !bytes.Equal(x.TotalExpenditure().Bytes(), OneBlockBigReward.Bytes()) {
		return fmt.Errorf("wrong block reward")
	}

	if !bytes.Equal(x.GetFee(), GetBig0Bytes()) {
		return fmt.Errorf("generate's fee should be ZERO")
	}

	publicKey := utils.Bytes2PublicKey(x.GetParticipants()[0])
	if err := x.Verify(publicKey); err != nil {
		return err
	}

	return nil
}

func (x *Tx) CheckRegister() error {
	if x.Header == nil {
		return errors.New("register is missing header")
	}

	if x.GetConvener() != 01 {
		return fmt.Errorf("register's convener should be 1")
	}

	if len(x.GetParticipants()) != 1 {
		return fmt.Errorf("register should have only one participant")
	}

	if len(x.GetValues()) != 1 {
		return fmt.Errorf("register should have only one value")
	}

	if !bytes.Equal(x.GetValues()[0], GetBig0Bytes()) {
		return fmt.Errorf("register should have only one 0 value")
	}

	if new(big.Int).SetBytes(x.GetFee()).Cmp(OneBlockBigReward) < 0 {
		return fmt.Errorf("register should have at least 10NG(one block reward) fee")
	}

	if len(x.GetExtra()) != 2<<3 {
		return fmt.Errorf("register should have uint64 little-endian bytes as extra")
	}

	publicKey := utils.Bytes2PublicKey(x.GetParticipants()[0])
	if err := x.Verify(publicKey); err != nil {
		return err
	}

	return nil
}

func (x *Tx) CheckLogout(key secp256k1.PublicKey) error {
	if x.Header == nil {
		return errors.New("logout is missing header")
	}

	if len(x.GetParticipants()) != 0 {
		return fmt.Errorf("logout should have NO participant")
	}

	if x.GetConvener() == 0 {
		return fmt.Errorf("logout's convener should NOT be 0")
	}

	if len(x.GetValues()) != 0 {
		return fmt.Errorf("logout should have NO value")
	}

	if len(x.GetValues()) != len(x.GetParticipants()) {
		return fmt.Errorf("logout should have same len with participants")
	}

	if err := x.Verify(key); err != nil {
		return err
	}

	return nil
}

func (x *Tx) CheckTransaction(key secp256k1.PublicKey) error {
	if x.Header == nil {
		return errors.New("transaction is missing header")
	}

	if x.GetConvener() == 0 {
		return fmt.Errorf("transaction's convener should NOT be 0")
	}

	if len(x.GetValues()) != len(x.GetParticipants()) {
		return fmt.Errorf("transaction should have same len with participants")
	}

	if err := x.Verify(key); err != nil {
		return err
	}

	return nil
}

func (x *Tx) CheckAssign(key secp256k1.PublicKey) error {
	if x.Header == nil {
		return errors.New("assign is missing header")
	}

	if x.GetConvener() == 0 {
		return fmt.Errorf("assign's convener should NOT be 0")
	}

	if len(x.GetParticipants()) != 0 {
		return fmt.Errorf("assign should have NO participant")
	}

	if len(x.GetValues()) != 0 {
		return fmt.Errorf("assign should have NO value")
	}

	if err := x.Verify(key); err != nil {
		return err
	}

	return nil
}

func (x *Tx) CheckAppend(key secp256k1.PublicKey) error {
	if x.Header == nil {
		return errors.New("append is missing header")
	}

	if len(x.GetParticipants()) != 0 {
		return fmt.Errorf("append should have NO participant")
	}

	if x.GetConvener() == 0 {
		return fmt.Errorf("append's convener should NOT be 0")
	}

	if len(x.GetValues()) != 0 {
		return fmt.Errorf("append should have NO value")
	}

	if err := x.Verify(key); err != nil {
		return err
	}

	return nil
}

// Signature will re-sign the Tx with private key.
func (x *Tx) Signature(privateKeys ...*secp256k1.PrivateKey) (err error) {
	b, err := utils.Proto.Marshal(x.Header)
	if err != nil {
		log.Error(err)
	}

	ds := make([]*big.Int, len(privateKeys))
	for i := range privateKeys {
		ds[i] = privateKeys[i].D
	}

	sign, err := schnorr.AggregateSignatures(ds, sha3.Sum256(b))
	if err != nil {
		log.Panic(err)
	}

	x.Sign = sign[:]

	return
}

func (x *Tx) GetType() TxType {
	return x.Header.GetType()
}

func (x *Tx) GetConvener() uint64 {
	return x.Header.GetConvener()
}

func (x *Tx) GetValues() [][]byte {
	return x.Header.GetValues()
}

func (x *Tx) GetParticipants() [][]byte {
	return x.Header.GetParticipants()
}

func (x *Tx) GetFee() []byte {
	return x.Header.GetFee()
}

func (x *Tx) GetNonce() uint64 {
	return x.Header.GetNonce()
}

func (x *Tx) GetExtra() []byte {
	return x.Header.GetExtra()
}

func (x *Tx) TotalExpenditure() *big.Int {
	total := GetBig0()

	for i := range x.Header.Values {
		total.Add(total, new(big.Int).SetBytes(x.Header.Values[i]))
	}

	return new(big.Int).Add(new(big.Int).SetBytes(x.Header.Fee), total)
}

// GetGenesisGenerateTx is a constructed function.
func GetGenesisGenerateTx() *Tx {
	gen := NewUnsignedTx(
		TxType_GENERATE,
		0,
		[][]byte{GenesisPublicKey},
		[]*big.Int{OneBlockBigReward},
		GetBig0(),
		1,
		nil,
	)

	gen.Sign = GenesisGenerateTxSign

	return gen
}
