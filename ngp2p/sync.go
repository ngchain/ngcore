package ngp2p

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/atomic"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

type forkManager struct {
	w       *wired
	enabled *atomic.Bool
}

func (w *wired) updateStatus() {
	total := 0
	synced := 0
	localHeight := w.node.consensus.GetLatestBlockHeight()

	w.node.RemoteHeights.Range(func(_, value interface{}) bool {
		total++
		if localHeight+ngtypes.BlockCheckRound >= value.(uint64) {
			synced++
		}
		return true
	})

	progress := float64(synced) / float64(total)
	w.node.isSyncedCh <- progress > 0.9
}

func (w *wired) syncLoop() {
	syncTicker := time.NewTicker(ngtypes.TargetTime)
	defer syncTicker.Stop()

	lastTimeIsSynced := false // default

	for {
		select {
		case <-syncTicker.C:
			for _, p := range w.node.Peerstore().Peers() {
				if p == w.node.ID() {
					continue
				}
				log.Debugf("pinging to %s", p)
				w.ping(p)
			}

			go w.updateStatus()

		case isSynced := <-w.node.isSyncedCh:
			if isSynced && !lastTimeIsSynced {
				log.Info("localnode is synced with network")
				if w.node.OnSynced != nil {
					w.node.OnSynced()
				}
			}

			if !isSynced && lastTimeIsSynced {
				log.Info("localnode is not synced with network, syncing...")
				if w.node.OnNotSynced != nil {
					w.node.OnNotSynced()
				}
			}

			lastTimeIsSynced = isSynced
		}
	}
}

func (s *forkManager) handlePong(remotePeerID peer.ID, pong *pb.PingPongPayload) {
	// ignite sync/fork
	if !s.enabled.Load() {
		s.enabled.Store(true)
	}

	if s.w.node.isStrictMode {
		log.Infof("start syncing with %s, finding last same block", remotePeerID)
	} else {
		log.Infof("start syncing with %s, forcing local chain switching", remotePeerID)
	}

	var from uint64
	to := pong.LatestHeight
	if to > ngtypes.BlockCheckRound {
		from = to - ngtypes.BlockCheckRound
	}

	go s.w.getChain(remotePeerID, from, to)
}

func (s *forkManager) handleChain(remotePeerID peer.ID, chain *pb.ChainPayload) {
	if s.enabled.Load() {
		if s.w.node.isStrictMode {
			// todo: maybe using trie will be better
			for i := len(chain.Blocks) - 1; i >= 0; i-- {
				hash, err := chain.Blocks[i].CalculateHash()
				if err != nil {
					log.Errorf("failed to hash block: %v: %s", chain.Blocks[i], err)
				}
				_, err = s.w.node.consensus.GetBlockByHash(hash)
				if err != nil {
					// not in local, continue searching
					continue
				}

				// got i
				err = s.w.node.consensus.PutNewChain(chain.Blocks[i:]...)
				if err != nil {
					log.Errorf("failed to putting chain: %s", err)
					return
				}

				localBlockHeight := s.w.node.consensus.GetLatestBlockHeight()
				if chain.LatestHeight > localBlockHeight {
					go s.w.getChain(remotePeerID, localBlockHeight, chain.LatestHeight)
					return
				}

				// finally done
				s.enabled.Store(false)
				return
			}

			// not found
			var from uint64
			to := chain.Blocks[0].GetHeight() - 1
			if to > ngtypes.BlockCheckRound {
				from = to - ngtypes.BlockCheckRound
			}

			go s.w.getChain(remotePeerID, from, to)
			return

		} else {
			log.Infof("start syncing with %s, forcing local chain switching", remotePeerID)
			err := s.w.node.consensus.SwitchTo(chain.Blocks...)
			if err != nil {
				log.Errorf("failed to putting chain: %s", err)
				return
			}

			localBlockHeight := s.w.node.consensus.GetLatestBlockHeight()
			if chain.LatestHeight > localBlockHeight {
				go s.w.getChain(remotePeerID, localBlockHeight, chain.LatestHeight)
				return
			}

			// finally done
			s.enabled.Store(false)
			return
		}
	}
}
