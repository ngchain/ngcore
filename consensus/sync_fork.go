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

	log.Warnf("start forking Chain from remote node %s, target height: %d", record.id, record.latest)
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

	ptr := localHeight

	// when the chainLen (the len of returned chain) is not equal to defaults.MaxBlocks, means it has reach the latest height
	for {
		if ptr == 0 {
			panic("cannot fork: completely different chains!")
		}

		roundHashes := uint64(defaults.MaxBlocks)
		if ptr < defaults.MaxBlocks {
			roundHashes = ptr
		}

		ptr -= roundHashes

		// get local hashes as params
		for h := ptr + 1; h <= ptr+roundHashes; h++ {
			b, err := mod.pow.Chain.GetBlockByHeight(h)
			if err != nil {
				// when gap is too large
				return nil, err
			}

			blockHashes[h-ptr] = b.Hash() // panic here
		}

		to := blockHashes[len(blockHashes)-1]

		// the to param shouldnt be nil or empty hash here
		// if nil, the len of returned chain will be default.MaxBlocks forever
		chain, err := mod.getRemoteChain(record.id, blockHashes, to)
		if err != nil {
			return nil, err
		}

		blocks = append(chain, blocks...)
		if uint64(len(chain)) != roundHashes {
			break
		}
	}

	return blocks, nil
}
