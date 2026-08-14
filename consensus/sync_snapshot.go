package consensus

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// fetchRemoteRange downloads the blocks [from, to] from the remote in
// MaxBlocks-sized rounds, without applying anything
func (mod *syncModule) fetchRemoteRange(record *RemoteRecord, from, to uint64) ([]*ngtypes.FullBlock, error) {
	blocks := make([]*ngtypes.FullBlock, 0, to-from+1)

	for cur := from; cur <= to; {
		roundTo := utils.MinUint64(cur+defaults.MaxBlocks-1, to)
		heightRange := bytes.Join([][]byte{
			utils.PackUint64LE(cur),
			utils.PackUint64LE(roundTo),
		}, nil)

		round, err := mod.getRemoteChain(record.id, [][]byte{}, heightRange)
		if err != nil {
			return nil, err
		}
		if len(round) == 0 {
			return nil, errors.Errorf("remote returned no blocks for [%d, %d]", cur, roundTo)
		}

		blocks = append(blocks, round...)
		cur = round[len(round)-1].GetHeight() + 1
	}

	return blocks, nil
}

// doSnapshotSync fast-syncs to the remote checkpoint: it downloads the
// chain segment and the checkpoint's state sheet, then applies both in
// one atomic step (Chain.ApplySnapshot) instead of the old force-apply
// plus multi-txn state rebuild. Blocks after the checkpoint arrive
// through the normal sync afterwards
func (mod *syncModule) doSnapshotSync(record *RemoteRecord) error {
	if mod.Locker.IsActive() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start snapshot syncing with remote node %s, target checkpoint %d", record.id, record.checkpointHeight)

	localHeight := mod.pow.Chain.GetLatestBlockHeight()
	if localHeight >= record.checkpointHeight {
		return nil // nothing to fast-forward
	}

	blocks, err := mod.fetchRemoteRange(record, localHeight+1, record.checkpointHeight)
	if err != nil {
		return err
	}

	sheet, err := mod.getRemoteStateSheet(record)
	if err != nil {
		return err
	}

	err = mod.pow.Chain.ApplySnapshot(blocks, sheet)
	if err != nil {
		return errors.Wrap(err, "failed to apply the snapshot")
	}

	log.Warnf("snapshot sync finished with remote node %s, local height %d", record.id, mod.pow.Chain.GetLatestBlockHeight())

	return nil
}

// doSnapshotConverging switches to the remote's heavier fork in snapshot
// mode: the branch since the fork point gets fetched, extended (or
// trimmed) to the remote checkpoint, and applied atomically together
// with the checkpoint's state sheet
func (mod *syncModule) doSnapshotConverging(record *RemoteRecord) error {
	if mod.Locker.IsActive() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start snapshot converging with remote node %s, target checkpoint %d", record.id, record.checkpointHeight)

	branch, err := mod.getBlocksForConverging(record)
	if err != nil {
		return err
	}
	if len(branch) == 0 {
		return errors.New("converging returned an empty branch")
	}

	// the sheet binds the remote checkpoint, so the applied segment must
	// end exactly there
	tipHeight := branch[len(branch)-1].GetHeight()
	switch {
	case tipHeight > record.checkpointHeight:
		cut := len(branch) - int(tipHeight-record.checkpointHeight)
		if cut <= 0 || branch[0].GetHeight() > record.checkpointHeight {
			return errors.Errorf("fork point@%d is above the remote checkpoint@%d",
				branch[0].GetHeight()-1, record.checkpointHeight)
		}
		branch = branch[:cut]
	case tipHeight < record.checkpointHeight:
		tail, err := mod.fetchRemoteRange(record, tipHeight+1, record.checkpointHeight)
		if err != nil {
			return err
		}
		branch = append(branch, tail...)
	}

	sheet, err := mod.getRemoteStateSheet(record)
	if err != nil {
		return err
	}

	err = mod.pow.Chain.ApplySnapshot(branch, sheet)
	if err != nil {
		return errors.Wrap(err, "failed to apply the snapshot")
	}

	log.Warn("snapshot converging finished")

	return nil
}
