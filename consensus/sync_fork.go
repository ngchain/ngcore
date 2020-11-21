package consensus

import (
	"bytes"
	"fmt"
	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
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

		var from = ptr + 1
		var to = ptr + roundHashes
		// get local hashes as params
		for h := from; h <= to; h++ {
			b, err := mod.pow.Chain.GetBlockByHeight(h)
			if err != nil {
				// when gap is too large
				return nil, err
			}

			blockHashes[h-ptr-1] = b.Hash()
		}

		// to == nil means fork mode
		chain, err := mod.getRemoteChain(record.id, blockHashes, bytes.Join([][]byte{utils.PackUint64LE(from), utils.PackUint64LE(to)}, nil))
		if err != nil {
			return nil, err
		}
		if chain == nil {
			// chain == nil means all hashes are matched
			localChain := make([]*ngtypes.Block, 0, defaults.MaxBlocks)
			for i := range blockHashes {
				block, err := mod.pow.Chain.GetBlockByHash(blockHashes[i])
				if err != nil {
					return nil, fmt.Errorf("failed on constructing local chain: %s", err)
				}
				localChain = append(localChain, block)
			}

			blocks = append(localChain, blocks...)
		} else {
			blocks = append(chain, blocks...)
			break
		}
	}

	return blocks, nil
}
