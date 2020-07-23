package consensus

import (
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// createGenerateTx will create a generate Tx for new Block.
// generate Tx is disallowed to edit external so use more local var
func (pow *PoWork) createGenerateTx(extraData []byte) *ngtypes.Tx {
	addr := ngtypes.NewAddress(pow.PrivateKey)
	gen := ngtypes.NewUnsignedTx(
		ngtypes.TxType_GENERATE,
		storage.GetChain().GetLatestBlockHash(),
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
