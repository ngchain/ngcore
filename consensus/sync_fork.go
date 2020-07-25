package consensus

import (
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// detectFork detection ignites the forking in local node
// then do a filter covering all remotes to get the longest chain (if length is same, choose the heavier latest block one)
func (mod *syncModule) detectFork() (shouldFork bool, remote *remoteRecord) {
	for _, r := range mod.store {
		if r.shouldFork() {
			return true, r
		}
	}

	return false, nil
}

// force local chain be same as the remote record
// fork is a danger operation so all msg are warn level
func (mod *syncModule) doFork(record *remoteRecord) error {
	pow.Lock()
	defer pow.Unlock()

	log.Warnf("start forking chain from remote node %s, height: %d", record.id, record.latest)
	chain, err := mod.getBlocksSinceForkPoint(record)
	if err != nil {
		return err
	}

	log.Warnf("have got the fork point: block@%d", chain[0].Height)
	err = mod.pow.forceApplyBlocks(chain)
	if err != nil {
		return err
	}

	// RULE: there are 3 choices
	// 1. regenerate the state(time-consuming)
	// 2. download the state from remote(maybe unreliable)
	// 3. flash back(require remove logout and assign tx)
	// Currently choose the No.1
	err = ngstate.GetStateManager().RegenerateState()
	if err != nil {
		return err
	}

	log.Warn("fork finished")
	return nil
}

// getBlocksSinceForkPoint gets the fork point by comparing hashes between local and remote
func (mod *syncModule) getBlocksSinceForkPoint(record *remoteRecord) ([]*ngtypes.Block, error) {
	blocks := make([]*ngtypes.Block, 0)
	blockHashes := make([][]byte, ngp2p.MaxBlocks)

	localHeight := storage.GetChain().GetLatestBlockHeight()

	chainLen := ngp2p.MaxBlocks

	for i := uint64(0); chainLen == ngp2p.MaxBlocks; i++ {
		for height := localHeight - (i+1)*ngp2p.MaxBlocks; height < localHeight-i*ngp2p.MaxBlocks; height++ {
			b, _ := storage.GetChain().GetBlockByHeight(height)

			blockHashes[height-(localHeight-ngp2p.MaxBlocks)] = b.Hash()
		}

		// requires protocol v0.0.3
		chain, err := mod.getRemoteChain(record.id, blockHashes, nil)
		if err != nil {
			return nil, err
		}

		chainLen = len(chain)
		blocks = append(chain, blocks...)
	}

	return blocks, nil
}

// forceApplyBlocks checks the block and then calls ngchain's PutNewBlock, after which update the state
func (pow *PoWork) forceApplyBlocks(blocks []*ngtypes.Block) error {
	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		if err := pow.checkBlock(block); err != nil {
			return err
		}

		err := storage.GetChain().ForcePutNewBlock(block)
		if err != nil {
			return err
		}
	}

	return nil
}
