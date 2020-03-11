package consensus

import (
	"crypto/ecdsa"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/txpool"
)

func InitConsensusManager(blockChain *chain.BlockChain, vaultChain *chain.VaultChain, sheetManager *sheetManager.SheetManager, privateKey *ecdsa.PrivateKey, txPool *txpool.TxPool) *Consensus {
	latestBlock := blockChain.GetLatestBlock()
	latestVault := vaultChain.GetLatestVault()

	c := &Consensus{
		BlockChain: blockChain,
		VaultChain: vaultChain,

		privateKey:   privateKey,
		SheetManager: sheetManager,

		CurrentBlock: latestBlock,
		CurrentVault: latestVault,

		TxPool: txPool,
	}

	return c
}
