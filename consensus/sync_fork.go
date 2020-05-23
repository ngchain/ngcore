package consensus

// detectFork detection ignites the forking in local node
// then do a filter covering all remotes to get the longest chain (if length is same, choose the heavier latest block one)
func (sync *syncModule) detectFork() bool {
	for _, r := range sync.store {
		if r.shouldFork() {
			return true
		}
	}

	return false
}

func (sync *syncModule) doFork() error {
	// TODO: implement me
	return nil
}
