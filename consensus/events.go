package consensus

func (pow *PoWork) loop() {
	for {
		if pow.localNode.OnBlock == nil || pow.localNode.OnTx == nil {
			panic("event chan is nil")
		}

		select {
		case block := <-pow.localNode.OnBlock:
			pow.PutNewBlock(block)
		case tx := <-pow.localNode.OnTx:
			pow.txpool.PutTxs(tx)
		}
	}
}
