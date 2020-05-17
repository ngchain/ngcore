package consensus

import (
	"fmt"
	"sort"

	core "github.com/libp2p/go-libp2p-core"

	"github.com/ngchain/ngcore/ngp2p"
)

func (sync *syncModule) bootstrap() {
	sync.RLock()
	// 
	for _, remotePeerID := range pow.localNode.Peerstore().Peers() {
		go sync.getRemoteStatus(remotePeerID)
	}

	slice := make([]*remoteRecord, len(sync.store))
	i := 0
	for _, v := range sync.store {
		slice[i] = v
		i++
	}
	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].lastChatTime > slice[j].lastChatTime
	})

	// initial sync
	for _, r := range slice {
		if r.shouldSync() {
			sync.doSync(r)
		}
	}
	sync.RUnlock()
}

// GetRemoteStatus just get the remote status from remote
func (sync *syncModule) getRemoteStatus(peerID core.PeerID) error {
	origin := pow.chain.GetOriginBlock()
	latest := pow.chain.GetLatestBlock()
	checkpointHash := pow.chain.GetLatestCheckpointHash()

	id, s := pow.localNode.Ping(peerID, origin.GetHeight(), latest.GetHeight(), checkpointHash)
	if s == nil {
		return fmt.Errorf("failed to send ping")
	}

	reply, err := ngp2p.ReceiveReply(id, s)
	if err != nil {
		return err
	}

	switch reply.Header.MessageType {
	case ngp2p.MessageType_PONG:
		pongPayload, err := ngp2p.DecodePongPayload(reply.Payload)
		if err != nil {
			return err
		}
		pow.syncMod.PutRemote(peerID, &remoteRecord{
			id:     peerID,
			origin: pongPayload.Origin,
			latest: pongPayload.Latest,
		})

	case ngp2p.MessageType_REJECT:
		return fmt.Errorf("ping is rejected by remote")
	default:
		return fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}

	return nil
}
