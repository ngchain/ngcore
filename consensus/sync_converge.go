package consensus

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// MustConverge detection ignites the forking in local node
// then do a filter covering all remotes to get the longest chain (if length is same, choose the heavier latest block one).
func (mod *syncModule) MustConverge(slice []*RemoteRecord) []*RemoteRecord {
	ret := make([]*RemoteRecord, 0)
	latestHeight := mod.pow.Chain.GetLatestBlockHeight()
	latestCheckPoint := mod.pow.Chain.GetLatestCheckpoint()

	for _, r := range slice {
		if r.shouldConverge(latestCheckPoint, latestHeight) {
			ret = append(ret, r)
		}
	}

	return ret
}

// force local chain be same as the remote record
// converge is a danger operation so all msg are warn level.
func (mod *syncModule) doConverging(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start converging chain from remote node %s, target height: %d", record.id, record.latest)
	chain, err := mod.getBlocksForConverging(record)
	if err != nil {
		return fmt.Errorf("failed to get blocks for converging: %w", err)
	}

	// localSamepoint, _ := mod.pow.Chain.GetBlockByHeight(chain[1].Height)
	// log.Warnf("have got the diffpoint: block@%d: local: %x remote %x", chain[1].Height, chain[1].Hash(), localSamepoint.Hash())

	err = mod.pow.Chain.ForceApplyBlocks(chain)
	if err != nil {
		return err
	}

	// RULE: there are 3 choices
	// 1. regenerate the state(time-consuming)
	// 2. download the state from remote(maybe unreliable)
	// 3. flash back(require remove destroy and assign tx)
	// Currently choose the No.1
	log.Warnf("regenerateing local state")
	err = mod.pow.State.RebuildFromBlockStore()
	if err != nil {
		return err
	}

	log.Warn("converging finished")
	return nil
}

// getBlocksForConverging gets the blocks since the diffpoint (inclusive) by comparing hashes between local and remote.
func (mod *syncModule) getBlocksForConverging(record *RemoteRecord) ([]*ngtypes.FullBlock, error) {
	blocks := make([]*ngtypes.FullBlock, 0)

	localHeight := mod.pow.Chain.GetLatestBlockHeight()
	localOriginHeight := mod.pow.Chain.GetOriginBlock().GetHeight()

	ptr := localHeight

	// when the chainLen (the len of returned chain) is not equal to defaults.MaxBlocks, means it has reach the latest height
	for {
		if ptr <= localOriginHeight {
			panic("converging failed: completely different chains!")
		}

		blockHashes := make([][]byte, 0, defaults.MaxBlocks)
		to := ptr
		roundHashes := utils.MinUint64(defaults.MaxBlocks, ptr-localOriginHeight)
		ptr -= roundHashes
		from := ptr + 1

		// get local hashes as params
		for h := from; h <= to; h++ {
			b, err := mod.pow.Chain.GetBlockByHeight(h)
			if err != nil {
				// when gap is too large
				return nil, err
			}

			blockHashes = append(blockHashes, b.GetHash())
		}

		// To == from+to means converging mode
		chain, err := mod.getRemoteChain(record.id, blockHashes, bytes.Join([][]byte{utils.PackUint64LE(from), utils.PackUint64LE(to)}, nil))
		if err != nil {
			return nil, err
		}
		if chain == nil {
			// chain == nil means all hashes are matched
			localChain := make([]*ngtypes.FullBlock, 0, defaults.MaxBlocks)
			for i := range blockHashes {
				block, err := mod.pow.Chain.GetBlockByHash(blockHashes[i])
				if err != nil {
					return nil, fmt.Errorf("failed on constructing local chain: %w", err)
				}
				localChain = append(localChain, block.(*ngtypes.FullBlock))
			}

			blocks = append(localChain, blocks...)
		} else {
			blocks = append(chain, blocks...)
			break
		}
	}

	return blocks, nil
}
