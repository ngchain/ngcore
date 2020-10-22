package consensus

import (
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// createGenerateTx will create a generate Tx for new Block.
// generate Tx is disallowed to edit external so use more local var
func (pow *PoWork) createGenerateTx(extraData []byte) *ngtypes.Tx {
	addr := ngtypes.NewAddress(pow.PrivateKey)
	gen := ngtypes.NewUnsignedTx(
		pow.Network,
		ngtypes.TxType_GENERATE,
		pow.Chain.GetLatestBlockHash(),
		0,
		[][]byte{addr},
		[]*big.Int{ngtypes.OneBlockBigReward},
		ngtypes.GetBig0(),
		extraData,
	)

	err := gen.Signature(pow.PrivateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}
