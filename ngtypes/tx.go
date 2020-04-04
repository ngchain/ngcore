package ngtypes

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"

	"github.com/cbergoon/merkletree"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/utils"
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
		log.Panic("unsigned transaction")
	}

	o := m.Copy()
	o.R = nil
	o.S = nil

	b, err := proto.Marshal(o)
	if err != nil {
		log.Error(err)
	}

	// hash := sha256.Sum256(b)
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

// BigIntsToBytesList is a helper converts bigInts to raw bytes slice
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

	publicKey := utils.Bytes2ECDSAPublicKey(m.GetParticipants()[0])

	if !m.Verify(publicKey) {
		return fmt.Errorf("failed to verify the generation with publicKey")
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

// Signature will re-sign the Tx with private key
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
	gen := NewUnsignedTransaction(
		0,
		0,
		[][]byte{GenesisPK},
		[]*big.Int{Big0},
		Big0,
		0,
		nil,
	)

	gen.HeaderHash, _ = gen.Header.CalculateHash()

	// FIXME: before init network should manually init the R & S
	gen.R, _ = hex.DecodeString("e96066f4d0317f8141c6e2969202a4eebb1dfba5fc979d20b7522e9cfedc126d")
	gen.S, _ = hex.DecodeString("112f459980b83fcff9416fa2cbf64a032baa537db4f0107e806b39bd8db385c6")

	return gen
}
