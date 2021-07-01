package consensus

import "fmt"

// MustSync will start a sync and stop until reaching the latest height
// RULE: checkpoint converging: when a node mined a checkpoint, all other node are forced to start sync.
func (mod *syncModule) MustSync(slice []*RemoteRecord) []*RemoteRecord {
	ret := make([]*RemoteRecord, 0)
	latestHeight := mod.pow.Chain.GetLatestBlockHeight()

	for _, r := range slice {
		if r.shouldSync(latestHeight) {
			ret = append(ret, r)
		}
	}

	return ret
}

func (mod *syncModule) doSync(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start syncing with remote node %s, target height %d", record.id, record.latest)

	// get chain
	for mod.pow.Chain.GetLatestBlockHeight() < record.latest {
		chain, err := mod.getRemoteChainFromLocalLatest(record)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = mod.pow.Chain.ApplyBlock(chain[i])
			if err != nil {
				return fmt.Errorf("failed on applying block@%d: %s", chain[i].Header.Height, err)
			}
		}
	}

	height := mod.pow.Chain.GetLatestBlockHeight()
	log.Warnf("sync finished with remote node %s, local height %d", record.id, height)

	return nil
}
