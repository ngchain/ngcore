package consensus

import (
	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
)

// detectFork detection ignites the forking in local node
// then do a filter covering all remotes to get the longest chain (if length is same, choose the heavier latest block one)
func (mod *syncModule) detectFork() (shouldFork bool, remote *remoteRecord) {
	latestHeight := mod.pow.Chain.GetLatestBlockHeight()
	latestCheckPoint := mod.pow.Chain.GetLatestCheckpoint()

	for _, r := range mod.store {
		if r.shouldFork(latestCheckPoint, latestHeight) {
			return true, r
		}
	}

	return false, nil
}

// force local chain be same as the remote record
// fork is a danger operation so all msg are warn level
func (mod *syncModule) doFork(record *remoteRecord) error {
	mod.pow.Lock()
	defer mod.pow.Unlock()

	log.Warnf("start forking Chain from remote node %s, height: %d", record.id, record.latest)
	chain, err := mod.getBlocksSinceForkPoint(record)
	if err != nil {
		return err
	}

	log.Warnf("have got the fork point: block@%d", chain[0].Height)
	err = mod.pow.Chain.ForceApplyBlocks(chain)
	if err != nil {
		return err
	}

	// RULE: there are 3 choices
	// 1. regenerate the state(time-consuming)
	// 2. download the state from remote(maybe unreliable)
	// 3. flash back(require remove logout and assign tx)
	// Currently choose the No.1
	err = mod.pow.State.Regenerate()
	if err != nil {
		return err
	}

	log.Warn("fork finished")
	return nil
}

// getBlocksSinceForkPoint gets the fork point by comparing hashes between local and remote
func (mod *syncModule) getBlocksSinceForkPoint(record *remoteRecord) ([]*ngtypes.Block, error) {
	blocks := make([]*ngtypes.Block, 0)
	blockHashes := make([][]byte, defaults.MaxBlocks)

	localHeight := mod.pow.Chain.GetLatestBlockHeight()

	chainLen := defaults.MaxBlocks

	for i := uint64(0); chainLen == defaults.MaxBlocks; i++ {
		for height := localHeight - (i+1)*defaults.MaxBlocks; height < localHeight-i*defaults.MaxBlocks; height++ {
			b, err := mod.pow.Chain.GetBlockByHeight(height)
			if err != nil {
				// when gap is too large
				return nil, err
			}

			blockHashes[height-(localHeight-defaults.MaxBlocks)] = b.Hash()
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
