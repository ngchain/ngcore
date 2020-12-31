package consensus

import (
	"sort"
	"time"
)

// main loop of sync module
func (mod *syncModule) loop() {
	ticker := time.NewTicker(10 * time.Second)

	for {
		<-ticker.C
		log.Infof("checking sync status")

		// do get status
		for _, id := range mod.localNode.Peerstore().Peers() {
			p, _ := mod.localNode.Peerstore().FirstSupportedProtocol(id, string(mod.localNode.GetWiredProtocol()))
			if p == string(mod.localNode.GetWiredProtocol()) && id != mod.localNode.ID() {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Warnf("failed to get remote status from %s: %s", id, err)
				}
			}
		}

		// do sync check takes the priority
		// convert map to slice first
		slice := make([]*RemoteRecord, len(mod.store))
		i := 0

		for _, v := range mod.store {
			slice[i] = v
			i++
		}

		sort.SliceStable(slice, func(i, j int) bool {
			return slice[i].lastChatTime > slice[j].lastChatTime
		})
		sort.SliceStable(slice, func(i, j int) bool {
			return slice[i].latest > slice[j].latest
		})

		var err error
		if records := mod.MustSync(slice); records != nil && len(records) != 0 {
			for _, record := range records {
				if mod.pow.SnapshotMode {
					err = mod.doSnapshotSync(record)
				} else {
					err = mod.doSync(record)
				}

				if err != nil {
					log.Warnf("do sync failed: %s, maybe require converging", err)
				} else {
					break
				}
			}
		}
		if err == nil {
			continue
		}

		if records := mod.MustConverge(slice); records != nil && len(records) != 0 {
			for _, record := range records {
				if mod.pow.SnapshotMode {
					err = mod.doSnapshotConverging(record)
				} else {
					err = mod.doConverging(record)
				}

				if err != nil {
					log.Errorf("converging failed: %s", err)
					record.recordFailure()
				} else {
					break
				}
			}
		}
	}
}
