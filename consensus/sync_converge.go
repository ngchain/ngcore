package consensus

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// MustConverge detection ignites the forking in local node
// then do a filter covering all remotes to get the longest chain (if length is same, choose the heavier latest block one).
func (mod *syncModule) MustConverge(slice []*RemoteRecord) []*RemoteRecord {
	// converging rewrites the local chain, so on public networks it
	// requires enough independent remotes to avoid being eclipsed by a
	// single malicious peer. The local regression net stays ungated
	if mod.pow.Network != ngtypes.ZERONET && len(slice) < minDesiredPeerCount {
		log.Warnf("converging suppressed: only %d remote(s) known, need %d", len(slice), minDesiredPeerCount)
		return nil
	}

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
	if mod.Locker.IsActive() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start converging chain from remote node %s, target height: %d", record.id, record.latest)
	chain, err := mod.getBlocksForConverging(record)
	if err != nil {
		return fmt.Errorf("failed to get blocks for converging: %w", err)
	}

	// SwitchToBranch validates the branch, rewrites the canonical chain
	// and replays the state all in ONE db txn: a failure (including
	// invalid remote blocks) leaves the local chain untouched
	err = mod.pow.Chain.SwitchToBranch(chain)
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

	// an origin-only chain has no fork to resolve: linear doSync handles it,
	// so converging refuses (and the loop below would have nothing to compare)
	if localHeight <= localOriginHeight {
		return nil, errors.New("converging failed: local chain is origin-only")
	}

	batchSize := mod.convergeBatchSize
	if batchSize == 0 {
		batchSize = defaults.MaxBlocks
	}

	// Walk local hashes backward in batches, asking the remote to locate the
	// most recent block it ALSO has (the fork point) and return its chain from
	// there up. When a batch has no common block, the fork point is older, so
	// keep walking back — the previous code broke on the remote's
	// non-attaching height-range reply, so a fork deeper than one batch never
	// converged. The oldest batch INCLUDES the origin: if the fork point is
	// the origin itself the remote finds it there; if even the origin does not
	// match, the chains are genuinely unrelated.
	to := localHeight
	for {
		var from uint64
		if to <= localOriginHeight+batchSize {
			from = localOriginHeight // last batch reaches the origin
		} else {
			from = to - batchSize + 1
		}

		blockHashes := make([][]byte, 0, to-from+1)
		for h := from; h <= to; h++ {
			b, err := mod.pow.Chain.GetBlockByHeight(h)
			if err != nil {
				return nil, err
			}
			blockHashes = append(blockHashes, b.GetHash())
		}

		// To == from‖to means converging mode
		chain, err := mod.getRemoteChain(record.id, blockHashes, bytes.Join([][]byte{utils.PackUint64LE(from), utils.PackUint64LE(to)}, nil))
		if err != nil {
			return nil, err
		}

		if len(chain) != 0 {
			// the remote returned its chain from the common point + 1, which
			// attaches to a block we already store: the divergent branch to
			// switch to (doSync then extends it to the remote tip)
			blocks = append(chain, blocks...)
			break
		}

		// no common block in this batch. If we already reached the origin,
		// the chains share nothing — give up. Otherwise walk further back.
		if from <= localOriginHeight {
			return nil, errors.New("converging failed: completely different chains")
		}
		to = from - 1
	}

	return blocks, nil
}
