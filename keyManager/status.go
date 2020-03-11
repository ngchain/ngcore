package keyManager

import (
	"math/big"
)

type Status struct {
	BlockHeight       uint64
	CurrentDifficulty *big.Int

	GenesisBlockHash []byte
	GenesisVaultHash []byte

	LatestBlockHash []byte
	LatestVaultHash []byte
}

//func (m *KeyManager) GetLocalStatus() *Status {
//
//	currentBlockHash, _ := m.Consensus.CurrentBlock.CalculateHash()
//	currentVaultHash, _ := m.Consensus.CurrentVault.CalculateHash()
//
//	return &Status{
//		BlockHeight:       m.Consensus.CurrentBlock.Header.Height,
//		CurrentDifficulty: new(big.Int).Div(ngtypes.MaxTarget, new(big.Int).SetBytes(m.Consensus.CurrentBlock.Header.Target)),
//
//		GenesisBlockHash: ngtypes.GenesisBlockHash,
//		GenesisVaultHash: ngtypes.GenesisVaultHash,
//
//		LatestBlockHash: currentBlockHash,
//		LatestVaultHash: currentVaultHash,
//	}
//}
//
//func (m *KeyManager) ImportBlocks(blocks []*ngtypes.Block) {
//	for index := 0; index < len(blocks); index++ {
//		go m.Consensus.GotNewBlock(blocks[index])
//	}
//}
//
//func (m *KeyManager) ImportOperations(txs []*ngtypes.Transaction) {
//	for index := 0; index < len(txs); index++ {
//		//go func(tx *ngtypes.Transaction) {
//		err := m.Consensus.SheetManager.ApplyTx(txs[index])
//		if err != nil {
//			log.Error(err)
//		}
//		//}(txs[index])
//	}
//}
