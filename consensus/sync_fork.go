package consensus


func (r *remoteRecord) shouldFork() {
	
}

// shouldFork detection ignites the forking in local node
// then do a filter covering all remotes to get the longest chain (if length is same, choose the heavier latest block one)
func (sync *syncModule) shouldFork() bool {
	for _, r := range sync.store {
		r.shouldFork()
	}
	return false
}

func (sync *syncModule) doFork() error {
	return nil
}
