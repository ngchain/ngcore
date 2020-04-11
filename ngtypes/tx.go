package ngtypes

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ngchain/go-schnorr"

	"github.com/gogo/protobuf/proto"
	"github.com/ngchain/secp256k1"
	"golang.org/x/crypto/sha3"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/utils"
)

var (
	ErrTxIsNotSigned         = errors.New("the transaction is not signed")
	ErrTxBalanceInsufficient = errors.New("balance is insufficient for payment")
	ErrTxWrongSign           = errors.New("the signer of transaction is not the own of the account")
)

// NewUnsignedTx will return an unsigned tx, must using Signature()
func NewUnsignedTx(txType TxType, convener uint64, participants [][]byte, values []*big.Int, fee *big.Int, nonce uint64, extraData []byte) *Tx {
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
		NetworkId: NetworkID,
		Header:    header,
		Sign:      nil,
	}
}

// IsSigned will return whether the op has been signed
func (m *Tx) IsSigned() bool {
	return m.Sign != nil
}

// Verify helps verify the transaction whether signed by the public key owner
func (m *Tx) Verify(publicKey secp256k1.PublicKey) error {
	if m.NetworkId != NetworkID {
		return fmt.Errorf("tx's network id is incorrect")
	}

	if m.Sign == nil {
		return fmt.Errorf("unsigned transaction")
	}

	if publicKey.X == nil || publicKey.Y == nil {
		return fmt.Errorf("illegal public key")
	}

	b, err := proto.Marshal(m.Header)
	if err != nil {
		return err
	}

	var signature [64]byte
	copy(signature[:], m.Sign)

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

// Bs58 is a tx's ReadableID in string
func (m *Tx) Bs58() string {
	b, err := proto.Marshal(m)
	if err != nil {
		log.Error(err)
	}
	return base58.FastBase58Encoding(b)
}

// HashHex is a tx's ReadableID in string
func (m *Tx) HashHex() string {
	b, err := m.CalculateHash()
	if err != nil {
		log.Error(err)
		return ""
	}

	return hex.EncodeToString(b)
}

// CalculateHash mainly for calculating the tire root of txs and sign tx
func (m *Tx) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		log.Error(err)
	}

	hash := sha3.Sum256(raw)
	return hash[:], nil
}

// Equals mainly for calculating the tire root of txs
func (m *Tx) Equals(other merkletree.Content) (bool, error) {
	tx, ok := other.(*Tx)
	if !ok {
		return false, errors.New("invalid transaction type")
	}

	otherHash, err := tx.Header.CalculateHash()
	if err != nil {
		return false, err
	}
	mHash, err := m.Header.CalculateHash()
	if err != nil {
		return false, err
	}

	return bytes.Equal(otherHash, mHash), nil
}

// TxsToMerkleTreeContents make a []merkletree.Content whose values is from txs
func TxsToMerkleTreeContents(txs []*Tx) []merkletree.Content {
	mtc := make([]merkletree.Content, len(txs))
	for i := range txs {
		mtc[i] = txs[i]
	}

	return mtc
}

func (m *Tx) Copy() *Tx {
	tx := proto.Clone(m).(*Tx)
	return tx
}

// BigIntsToBytesList is a helper converts bigInts to raw bytes slice
func BigIntsToBytesList(bigInts []*big.Int) [][]byte {
	bytesList := make([][]byte, len(bigInts))
	for i := 0; i < len(bigInts); i++ {
		bytesList[i] = bigInts[i].Bytes()
	}
	return bytesList
}

func (m *Tx) CheckGenerate() error {
	if m.Header == nil {
		return errors.New("generate is missing header")
	}

	if m.GetConvener() != 0 {
		return fmt.Errorf("generate's convener should be 0")
	}

	if len(m.GetValues()) != len(m.GetParticipants()) {
		return fmt.Errorf("generate should have same len with participants")
	}

	if !bytes.Equal(m.TotalExpenditure().Bytes(), OneBlockReward.Bytes()) {
		return fmt.Errorf("wrong block reward")
	}

	if !bytes.Equal(m.GetFee(), GetBig0Bytes()) {
		return fmt.Errorf("generate's fee should be ZERO")
	}

	publicKey := utils.Bytes2PublicKey(m.GetParticipants()[0])
	if err := m.Verify(publicKey); err != nil {
		return err
	}

	return nil
}

func (m *Tx) CheckRegister() error {
	if m.Header == nil {
		return errors.New("register is missing header")
	}

	if m.GetConvener() != 0 {
		return fmt.Errorf("register's convener should be 0")
	}

	if len(m.GetParticipants()) != 1 {
		return fmt.Errorf("register should have only one participant")
	}

	if len(m.GetValues()) != 1 {
		return fmt.Errorf("register should have only one value")
	}

	if !bytes.Equal(m.GetValues()[0], GetBig0Bytes()) {
		return fmt.Errorf("register should have only one 0 value")
	}

	if new(big.Int).SetBytes(m.GetFee()).Cmp(new(big.Int).Mul(NG, big.NewInt(10))) < 0 {
		return fmt.Errorf("register should have at least 10NG fee")
	}

	if len(m.GetExtra()) != 8 {
		return fmt.Errorf("register should have uint64 little-endian bytes as extra")
	}

	publicKey := utils.Bytes2PublicKey(m.GetParticipants()[0])
	if err := m.Verify(publicKey); err != nil {
		return err
	}

	return nil
}

func (m *Tx) CheckLogout(key secp256k1.PublicKey) error {
	if m.Header == nil {
		return errors.New("logout is missing header")
	}

	if len(m.GetParticipants()) != 0 {
		return fmt.Errorf("logout should have NO participant")
	}

	if m.GetConvener() == 0 {
		return fmt.Errorf("logout's convener should NOT be 0")
	}

	if len(m.GetValues()) != 0 {
		return fmt.Errorf("logout should have NO value")
	}

	if len(m.GetValues()) != len(m.GetParticipants()) {
		return fmt.Errorf("logout should have same len with participants")
	}

	if err := m.Verify(key); err != nil {
		return err
	}

	return nil
}

func (m *Tx) CheckTransaction(key secp256k1.PublicKey) error {
	if m.Header == nil {
		return errors.New("transaction is missing header")
	}

	if m.GetConvener() == 0 {
		return fmt.Errorf("transaction's convener should NOT be 0")
	}

	if len(m.GetValues()) != len(m.GetParticipants()) {
		return fmt.Errorf("transaction should have same len with participants")
	}

	if err := m.Verify(key); err != nil {
		return err
	}

	return nil
}

func (m *Tx) CheckAssign(key secp256k1.PublicKey) error {
	if m.Header == nil {
		return errors.New("assign is missing header")
	}

	if m.GetConvener() == 0 {
		return fmt.Errorf("assign's convener should NOT be 0")
	}

	if len(m.GetParticipants()) != 0 {
		return fmt.Errorf("assign should have NO participant")
	}

	if len(m.GetValues()) != 0 {
		return fmt.Errorf("assign should have NO value")
	}

	if err := m.Verify(key); err != nil {
		return err
	}

	return nil
}

func (m *Tx) CheckAppend(key secp256k1.PublicKey) error {
	if m.Header == nil {
		return errors.New("append is missing header")
	}

	if len(m.GetParticipants()) != 0 {
		return fmt.Errorf("append should have NO participant")
	}

	if m.GetConvener() == 0 {
		return fmt.Errorf("append's convener should NOT be 0")
	}

	if len(m.GetValues()) != 0 {
		return fmt.Errorf("append should have NO value")
	}

	if err := m.Verify(key); err != nil {
		return err
	}

	return nil
}

// Signature will re-sign the Tx with private key
func (m *Tx) Signature(privateKeys ...*secp256k1.PrivateKey) (err error) {
	b, err := proto.Marshal(m.Header)
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

	m.Sign = sign[:]

	return
}

func (m *Tx) GetType() TxType {
	return m.Header.GetType()
}

func (m *Tx) GetConvener() uint64 {
	return m.Header.GetConvener()
}

func (m *Tx) GetValues() [][]byte {
	return m.Header.GetValues()
}

func (m *Tx) GetParticipants() [][]byte {
	return m.Header.GetParticipants()
}

func (m *Tx) GetFee() []byte {
	return m.Header.GetFee()
}

func (m *Tx) GetNonce() uint64 {
	return m.Header.GetNonce()
}

func (m *Tx) GetExtra() []byte {
	return m.Header.GetExtra()
}

func (m *Tx) TotalExpenditure() *big.Int {
	total := GetBig0()
	for i := range m.Header.Values {
		total.Add(total, new(big.Int).SetBytes(m.Header.Values[i]))
	}

	return new(big.Int).Add(new(big.Int).SetBytes(m.Header.Fee), total)
}

// GetGenesisGenerateTx is a constructed function
func GetGenesisGenerateTx() *Tx {
	gen := NewUnsignedTx(
		TX_GENERATE,
		0,
		[][]byte{GenesisPublicKey},
		[]*big.Int{OneBlockReward},
		GetBig0(),
		1,
		nil,
	)

	gen.Sign = GenesisGenerateTxSign

	return gen
}
