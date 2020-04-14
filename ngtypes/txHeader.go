package ngtypes

import (
	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/utils"
)

// CalculateHash mainly for calculating the tire root of txs and sign tx
func (m *TxHeader) CalculateHash() ([]byte, error) {
	raw, err := utils.Proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	hash := sha3.Sum256(raw)
	return hash[:], nil
}
