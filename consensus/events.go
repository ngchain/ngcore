package consensus

func (pow *PoWork) loop() {
	for {
		select {
		case block := <-pow.localNode.OnBlock:
			pow.PutNewBlock(block)
		case tx := <-pow.localNode.OnTx:
			pow.txpool.PutTxs(tx)
		}
	}
}
