package ngtypes

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"golang.org/x/crypto/sha3"
	"math/big"
)

// Sign will re-sign the Tx with private key
func (m *TxHeader) Signature(privKey *ecdsa.PrivateKey) (R, S *big.Int, err error) {
	b, err := m.Marshal()
	if err != nil {
		log.Error(err)
	}

	R, S, err = ecdsa.Sign(rand.Reader, privKey, b)
	if err != nil {
		log.Panic(err)
	}

	return
}

func (m *TxHeader) Check() error {
	if len(m.Participants) != len(m.Values) {
		return errors.New("participants count is not equals to values'")
	}
	return nil
}

func (m *TxHeader) CalculateHash() ([]byte, error) {
	b, err := m.Marshal()
	if err != nil {
		log.Error(err)
	}
	hash := sha3.Sum256(b)
	return hash[:], nil
}

func (m *TxHeader) TotalCharge() *big.Int {
	totalValue := Big0
	for i := range m.Values {
		totalValue.Add(totalValue, new(big.Int).SetBytes(m.Values[i]))
	}

	return new(big.Int).Add(new(big.Int).SetBytes(m.Fee), totalValue)
}
