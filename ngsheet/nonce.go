package ngsheet

func (m *Manager) GetNextNonce(accountID uint64) uint64 {
	m.accountsMu.RLock()
	defer m.accountsMu.RUnlock()

	return m.accounts[accountID].GetNonce() + 1
}
