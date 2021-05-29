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
	"google.golang.org/protobuf/proto"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"github.com/ngchain/ngcore/utils"
)

// Errors for Tx
var (
	// ErrTxWrongSign occurs when the signature of the Tx doesnt match the Tx 's caller/account
	ErrTxWrongSign = errors.New("the signer of transaction is not the own of the account")
)

type Tx struct {
	*ngproto.Tx
	Hash []byte
}

// NewTx is the default constructor for ngtypes.Tx
func NewTx(network ngproto.NetworkType, txType ngproto.TxType, prevBlockHash []byte,
	convener uint64, participants [][]byte, values [][]byte, fee,
	extraData, sign []byte) *Tx {
	return &Tx{
		Tx: &ngproto.Tx{
			Network:       network,
			Type:          txType,
			PrevBlockHash: prevBlockHash,
			Convener:      convener,
			Participants:  participants,
			Fee:           fee,
			Values:        values,
			Extra:         extraData,
			Sign:          sign,
		},
		Hash: nil,
	}
}

// NewTxFromProto implement the Tx from its parent
func NewTxFromProto(protoTx *ngproto.Tx) *Tx {
	return &Tx{
		Tx:   protoTx,
		Hash: nil,
	}
}

// NewUnsignedTx will return an unsigned tx, must using Signature().
func NewUnsignedTx(network ngproto.NetworkType, txType ngproto.TxType, prevBlockHash []byte,
	convener uint64, participants [][]byte, values []*big.Int, fee *big.Int,
	extraData []byte) *Tx {

	return NewTx(network, txType, prevBlockHash, convener, participants, BigIntsToBytesList(values), fee.Bytes(), extraData, nil)
}

// GetProto will return Tx's parent
func (x *Tx) GetProto() *ngproto.Tx {
	return x.Tx
}

func (*Tx) ProtoMessage() error {
	return fmt.Errorf("not a proto")
}

func (x *Tx) Marshal() ([]byte, error) {
	protoTx := proto.Clone(x.GetProto()).(*ngproto.Tx)

	return proto.Marshal(protoTx)
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
	b, err := x.Marshal()
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
	if x.Hash == nil {
		tx := proto.Clone(x.Tx).(*ngproto.Tx)
		tx.Sign = nil

		raw, err := x.Marshal()
		if err != nil {
			return nil, err
		}

		hash := sha3.Sum256(raw)

		x.Hash = hash[:]
	}

	return x.Hash, nil
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

	if !bytes.Equal(x.PrevBlockHash, tx.PrevBlockHash) {
		return false, nil
	}

	if !utils.BytesListEquals(x.Participants, tx.Participants) {
		return false, nil
	}

	if !utils.BytesListEquals(x.Values, tx.Values) {
		return false, nil
	}

	if !bytes.Equal(x.Fee, tx.Fee) {
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

// txsToMerkleTreeContents make a []merkletree.Content whose values is from txs.
func txsToMerkleTreeContents(txs []*Tx) []merkletree.Content {
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
func (x *Tx) CheckGenerate(blockHeight uint64) error {
	if x == nil {
		return errors.New("generate is missing header")
	}

	if x.GetConvener() != 0 {
		return fmt.Errorf("generate's convener should be 0")
	}

	if len(x.GetValues()) != len(x.GetParticipants()) {
		return fmt.Errorf("generate should have same len with participants")
	}

	if !(x.TotalExpenditure().Cmp(GetBlockReward(blockHeight)) == 0) {
		return fmt.Errorf("wrong block reward: expect %s but value is %s", GetBlockReward(blockHeight), x.TotalExpenditure())
	}

	if !bytes.Equal(x.GetFee(), big.NewInt(0).Bytes()) {
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

	if !bytes.Equal(x.GetValues()[0], big.NewInt(0).Bytes()) {
		return fmt.Errorf("register should have only one 0 value")
	}

	if new(big.Int).SetBytes(x.GetFee()).Cmp(RegisterFee) < 0 {
		return fmt.Errorf("register should have at least 10NG(one block reward) fee")
	}

	if len(x.GetExtra()) != 1<<3 {
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

	// check this on chain
	//var appendExtra AppendExtra
	//err = proto.Unmarshal(x.Extra, &appendExtra)
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

	if x.GetConvener() == 0 {
		return fmt.Errorf("deleteTx's convener should NOT be 0")
	}

	if len(x.GetParticipants()) != 0 {
		return fmt.Errorf("deleteTx should have NO participant")
	}

	if len(x.GetValues()) != 0 {
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

// TotalExpenditure helps calculate the total expenditure which the tx caller should pay
func (x *Tx) TotalExpenditure() *big.Int {
	total := big.NewInt(0)

	for i := range x.Values {
		total.Add(total, new(big.Int).SetBytes(x.Values[i]))
	}

	return new(big.Int).Add(new(big.Int).SetBytes(x.Fee), total)
}
