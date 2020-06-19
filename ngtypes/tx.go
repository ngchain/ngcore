package ngtypes

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ngchain/go-schnorr"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/secp256k1"
	"golang.org/x/crypto/sha3"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/utils"
)

// Errors for Tx
var (
	ErrTxWrongSign = errors.New("the signer of transaction is not the own of the account")
)

// NewUnsignedTx will return an unsigned tx, must using Signature().
func NewUnsignedTx(txType TxType, prevBlockHash []byte, convener uint64, participants [][]byte, values []*big.Int, fee *big.Int, extraData []byte) *Tx {

	return &Tx{
		Network:       NETWORK,
		Type:          txType,
		PrevBlockHash: prevBlockHash,
		Convener:      convener,
		Participants:  participants,
		Fee:           fee.Bytes(),
		Values:        BigIntsToBytesList(values),
		Extra:         extraData,
		Sign:          nil,
	}
}

// IsSigned will return whether the op has been signed.
func (x *Tx) IsSigned() bool {
	return x.Sign != nil
}

// Verify helps verify the transaction whether signed by the public key owner.
func (x *Tx) Verify(publicKey secp256k1.PublicKey) error {
	if x.Network != NETWORK {
		return fmt.Errorf("tx's network id is incorrect")
	}

	if x.Sign == nil {
		return fmt.Errorf("unsigned transaction")
	}

	if publicKey.X == nil || publicKey.Y == nil {
		return fmt.Errorf("illegal public key")
	}

	cp := proto.Clone(x).(*Tx)
	cp.Sign = nil
	b, err := utils.Proto.Marshal(cp)
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

// Hash mainly for calculating the tire root of txs and sign tx.
func (x *Tx) Hash() []byte {
	raw, err := utils.Proto.Marshal(x)
	if err != nil {
		panic(err)
	}

	hash := sha3.Sum256(raw)

	return hash[:]
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *Tx) CalculateHash() ([]byte, error) {
	raw, err := utils.Proto.Marshal(x)
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

	otherRawHeader, err := utils.Proto.Marshal(tx)
	if err != nil {
		return false, err
	}

	otherHash := sha3.Sum256(otherRawHeader)

	selfRawHeader, err := utils.Proto.Marshal(x)
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

// CheckGenerate does a self check for generate tx
func (x *Tx) CheckGenerate() error {
	if x == nil {
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

	publicKey := Address(x.GetParticipants()[0]).PubKey()
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

	publicKey := Address(x.GetParticipants()[0]).PubKey()
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

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckTransaction does a self check for normal transaction tx
func (x *Tx) CheckTransaction(publicKey secp256k1.PublicKey) error {
	if x == nil {
		return errors.New("transaction is missing header")
	}

	if x.GetConvener() == 0 {
		return fmt.Errorf("transaction's convener should NOT be 0")
	}

	if len(x.GetValues()) != len(x.GetParticipants()) {
		return fmt.Errorf("transaction should have same len with participants")
	}

	err := x.Verify(publicKey)
	if err != nil {
		return err
	}

	return nil
}

// CheckAssign does a self check for assign tx
func (x *Tx) CheckAssign(publicKey secp256k1.PublicKey) error {
	if x == nil {
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

	if len(x.GetParticipants()) != 0 {
		return fmt.Errorf("append should have NO participant")
	}

	if x.GetConvener() == 0 {
		return fmt.Errorf("append's convener should NOT be 0")
	}

	if len(x.GetValues()) != 0 {
		return fmt.Errorf("append should have NO value")
	}

	err := x.Verify(key)
	if err != nil {
		return err
	}

	return nil
}

// Signature will re-sign the Tx with private key.
func (x *Tx) Signature(privateKeys ...*secp256k1.PrivateKey) (err error) {
	tx := proto.Clone(x).(*Tx)
	tx.Sign = nil

	b, err := utils.Proto.Marshal(tx)
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

// TotalExpenditure helps calculate the total expenditure which the tx caller should pay
func (x *Tx) TotalExpenditure() *big.Int {
	total := GetBig0()

	for i := range x.Values {
		total.Add(total, new(big.Int).SetBytes(x.Values[i]))
	}

	return new(big.Int).Add(new(big.Int).SetBytes(x.Fee), total)
}

func GetGenesisGenerateTx() *Tx {
	ggtx := NewUnsignedTx(
		TxType_GENERATE,
		nil,
		0,
		[][]byte{GenesisAddress},
		[]*big.Int{OneBlockBigReward},
		GetBig0(),
		nil,
	)

	ggtx.Sign = GenesisGenerateTxSign

	return ggtx
}
