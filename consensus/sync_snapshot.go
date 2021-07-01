package consensus

import (
	"fmt"
)

// online available for initial sync
// TODO: init from a checkpoint, not genesisblock.
func (mod *syncModule) doSnapshotSync(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start snapshot syncing with remote node %s, target height %d", record.id, record.latest)

	for mod.pow.Chain.GetLatestBlockHeight() < record.checkpointHeight {
		chain, err := mod.getRemoteChainFromLocalLatest(record)
		if err != nil {
			return err
		}

		err = mod.pow.Chain.ForceApplyBlocks(chain)
		if err != nil {
			return err
		}
	}

	sheet, err := mod.getRemoteStateSheet(record)
	if err != nil {
		return err
	}
	err = mod.pow.State.RebuildFromSheet(sheet)
	if err != nil {
		return fmt.Errorf("failed on rebuilding state with sheet %x: %s", sheet.BlockHash, err)
	}

	height := mod.pow.Chain.GetLatestBlockHeight()
	log.Warnf("snapshot sync finished with remote node %s, local height %d", record.id, height)

	return nil
}

func (mod *syncModule) doSnapshotConverging(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start converging chain from remote node %s, target height: %d", record.id, record.latest)
	chain, err := mod.getBlocksForConverging(record)
	if err != nil {
		return err
	}

	localSamepoint, _ := mod.pow.Chain.GetBlockByHeight(chain[1].Header.Height)
	log.Warnf("have got the diffpoint: block@%d: local: %x remote %x", chain[1].Header.Height, chain[1].GetHash(), localSamepoint.GetHash())

	err = mod.pow.Chain.ForceApplyBlocks(chain)
	if err != nil {
		return err
	}

	// RULE: there are 3 choices
	// 1. regenerate the state(time-consuming)
	// 2. download the state from remote(maybe unreliable)
	// 3. flash back(require remove logout and assign tx)
	// Currently choose the No.1
	sheet, err := mod.getRemoteStateSheet(record)
	if err != nil {
		return err
	}
	log.Warnf("regenerateing local state")
	err = mod.pow.State.RebuildFromSheet(sheet)
	if err != nil {
		return err
	}

	log.Warn("converging finished")
	return nil
}
