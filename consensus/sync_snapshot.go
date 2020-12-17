package consensus

// TODO
func (mod *syncModule) doSnapshotSync(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start snapshot syncing with remote node %s, target height %d", record.id, record.latest)

	// get chain
	panic("WIP")

	height := mod.pow.Chain.GetLatestBlockHeight()
	log.Warnf("snapshot sync finished with remote node %s, local height %d", record.id, height)

	return nil
}

// TODO
func (mod *syncModule) doSnapshotConverging(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start snapshot converging with remote node %s, target height %d", record.id, record.latest)

	// get chain
	panic("WIP")

	height := mod.pow.Chain.GetLatestBlockHeight()
	log.Warnf("snapshot converging finished with remote node %s, local height %d", record.id, height)

	return nil
}
