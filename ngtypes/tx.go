package ngtypes

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"
	"math/big"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"
)

var (
	ErrTxInvalidNonce        = errors.New("the nonce in transaction is smaller than the account's record")
	ErrTxIsNotSigned         = errors.New("the transaction is not signed")
	ErrTxBalanceInsufficient = errors.New("balance is insufficient for payment")
	ErrTxWrongSign           = errors.New("the signer of transaction is not the own of the account")
	ErrTxMalformed           = errors.New("the transaction structure is malformed")
)

// types:
// 0 = generation
// 1 = tx
// 2= state(contract)

// NewUnsignedTransaction will return an Unsigned Operation, must using Signature()
func NewUnsignedTransaction(txType int32, convener uint64, participants [][]byte, values []*big.Int, fee *big.Int, nonce uint64, extraData []byte) *Transaction {
	header := &TxHeader{
		Version:      Version,
		Type:         txType,
		Convener:     convener,
		Participants: participants,
		Fee:          fee.Bytes(),
		Values:       BigIntsToBytesList(values),
		Nonce:        nonce,
		Extra:        extraData,
	}

	hash, _ := header.CalculateHash()

	return &Transaction{
		Header:     header,
		HeaderHash: hash,

		R: nil,
		S: nil,
	}
}

// IsSigned will return whether the op has been signed
func (m *Transaction) IsSigned() bool {
	if m.R == nil || m.S == nil {
		return false
	}
	return true
}

// Verify helps verify the operation whether signed by the public key owner
func (m *Transaction) Verify(pubKey ecdsa.PublicKey) bool {
	if m.R == nil || m.S == nil {
		log.Panic("unsigned operation")
	}

	o := m.Copy()
	o.R = nil
	o.S = nil

	b, err := proto.Marshal(o)
	if err != nil {
		log.Error(err)
	}

	return ecdsa.Verify(&pubKey, b, new(big.Int).SetBytes(m.R), new(big.Int).SetBytes(m.S))
}

// Bs58 is a tx's ReadableID in string
func (m *Transaction) Bs58() string {
	b, err := proto.Marshal(m)
	if err != nil {
		log.Error(err)
	}
	return base58.FastBase58Encoding(b)
}

// HashHex is a tx's ReadableID in string
func (m *Transaction) HashHex() string {
	b, err := m.CalculateHash()
	if err != nil {
		log.Error(err)
		return ""
	}

	return hex.EncodeToString(b)
}

// CalculateHash mainly for calculating the tire root of txs and sign tx
func (m *Transaction) CalculateHash() ([]byte, error) {
	raw, err := m.Marshal()
	if err != nil {
		log.Error(err)
	}

	hash := sha3.Sum256(raw)
	return hash[:], nil
}

// Equals mainly for calculating the tire root of txs
func (m *Transaction) Equals(other merkletree.Content) (bool, error) {
	var equal = true
	tx, ok := other.(*Transaction)
	if !ok {
		return false, errors.New("invalid operation type")
	}

	equal = equal && bytes.Equal(tx.HeaderHash, m.HeaderHash)
	//equal = equal && reflect.DeepEqual(tx, m)

	return equal, nil
}

func TxsToMerkleTreeContents(txs []*Transaction) []merkletree.Content {
	mtc := make([]merkletree.Content, len(txs))
	for i := range txs {
		mtc[i] = txs[i]
	}

	return mtc
}

func (m *Transaction) Copy() *Transaction {
	tx := proto.Clone(m).(*Transaction)
	return tx
}

func BigIntsToBytesList(bigInts []*big.Int) [][]byte {
	bytesList := make([][]byte, len(bigInts))
	for i := 0; i < len(bigInts); i++ {
		bytesList[i] = bigInts[i].Bytes()
	}
	return bytesList
}

// CheckTx checks normal tx. publicKey should get from sheet
func (m *Transaction) CheckTx(publicKey ecdsa.PublicKey) error {
	if m.GetConvener() == 0 {
		return fmt.Errorf("tx's convener should not be 0")
	}

	if m.Header == nil {
		return errors.New("tx is missing header")
	}

	if !m.Verify(publicKey) {
		return fmt.Errorf("failed to verify the tx with publicKey")
	}

	return nil
}

func (m *Transaction) CheckGen() error {
	if m.Header == nil {
		return errors.New("generation is missing header")
	}

	if len(m.GetParticipants()) != 1 {
		return fmt.Errorf("generation should have only one participant")
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), m.GetParticipants()[0])
	publicKey := ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	if !m.Verify(publicKey) {
		return fmt.Errorf("failed to verify the tx with publicKey")
	}

	if m.GetConvener() != 0 {
		return fmt.Errorf("generation's convener should be 0")
	}

	if len(m.GetValues()) != 1 {
		return fmt.Errorf("generation should have only one value")
	}

	if new(big.Int).SetBytes(m.GetValues()[0]).Cmp(OneBlockReward) != 0 {
		return fmt.Errorf("wrong block reward")
	}

	return nil
}

// Sign will re-sign the Tx with private key
func (m *Transaction) Signature(privKey *ecdsa.PrivateKey) (err error) {
	b, err := proto.Marshal(m)
	if err != nil {
		log.Error(err)
	}

	r, s, err := ecdsa.Sign(rand.Reader, privKey, b)
	if err != nil {
		log.Panic(err)
	}

	m.R = r.Bytes()
	m.S = s.Bytes()

	return
}

func (m *Transaction) GetType() int32 {
	return m.Header.GetType()
}

func (m *Transaction) GetConvener() uint64 {
	return m.Header.GetConvener()
}

func (m *Transaction) GetValues() [][]byte {
	return m.Header.GetValues()
}

func (m *Transaction) GetParticipants() [][]byte {
	return m.Header.GetParticipants()
}

func (m *Transaction) GetFee() []byte {
	return m.Header.GetFee()
}

func (m *Transaction) GetNonce() uint64 {
	return m.Header.GetNonce()
}

func (m *Transaction) GetVersion() int32 {
	return m.Header.GetVersion()
}

func (m *Transaction) GetExtra() []byte {
	return m.Header.GetExtra()
}

func (m *Transaction) TotalCharge() *big.Int {
	return m.Header.TotalCharge()
}

func GetGenesisGeneration() *Transaction {
	header := &TxHeader{
		Version:      Version,
		Type:         0,
		Convener:     0,
		Participants: [][]byte{GenesisPK},
		Fee:          Big0Bytes,
		Values: [][]byte{
			OneBlockReward.Bytes(),
		},
		Nonce: 0,
		Extra: nil,
	}

	headerHash, _ := proto.Marshal(header)

	// FIXME: before init network should manually init the R & S
	r, _ := hex.DecodeString("db60cdda46c5c4efb1eadd797b27bc785a713c16b5e33d92010cf1828855e577")
	s, _ := hex.DecodeString("f28ec61c9ec8e889377c34e8359b25f355500b15189c1c7f3f1f2fff61eb7873")

	return &Transaction{
		Header:     header,
		HeaderHash: headerHash,
		R:          r,
		S:          s,
	}
}

// TotalFee is a helper which helps calc the total fee among the ops
func TotalFee(txs []*Transaction) (totalFee *big.Int) {
	totalFee = big.NewInt(0)
	for _, tx := range txs {
		totalFee = new(big.Int).Add(totalFee, new(big.Int).SetBytes(tx.Header.Fee))
	}

	return
}
