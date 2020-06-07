package consensus

import (
	"github.com/ngchain/ngcore/storage"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// createGenerateTx will create a generate Tx for new Block.
// generate Tx is disallowed to edit external so use more local var
func (pow *PoWork) createGenerateTx(extraData []byte) *ngtypes.Tx {
	publicKeyBytes := utils.PublicKey2Bytes(*pow.PrivateKey.PubKey())
	gen := ngtypes.NewUnsignedTx(
		ngtypes.TxType_GENERATE,
		storage.GetChain().GetLatestBlockHash(),
		0,
		[][]byte{publicKeyBytes},
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
