package consensus

import (
	"fmt"
	"sort"

	"context"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multiaddr"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/storage"
)

func (sync *syncModule) bootstrap() {
	c := context.Background()

	peerStore := ngp2p.GetLocalNode().Peerstore()

	for i := range ngp2p.BootstrapNodes {
		addr, err := multiaddr.NewMultiaddr(ngp2p.BootstrapNodes[i])
		if err != nil {
			panic(err)
		}

		p, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			panic(err)
		}

		err = ngp2p.GetLocalNode().Connect(c, *p)
		if err != nil {
			panic(err)
		}

		peerStore.AddAddr(p.ID, addr, peerstore.PermanentAddrTTL)
	}

	for _, id := range peerStore.Peers() {
		if id != ngp2p.GetLocalNode().ID() {
			err := sync.getRemoteStatus(id)
			if err != nil {
				panic(err)
			}
		}
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
			err := sync.doInit(r)
			if err != nil {
				panic(err)
			}
		}
	}
}

// GetRemoteStatus just get the remote status from remote, and then put it into sync.store
func (sync *syncModule) getRemoteStatus(peerID core.PeerID) error {
	origin := storage.GetChain().GetOriginBlock()
	latest := storage.GetChain().GetLatestBlock()
	checkpointHash := storage.GetChain().GetLatestCheckpointHash()

	id, stream := ngp2p.GetLocalNode().Ping(peerID, origin.GetHeight(), latest.GetHeight(), checkpointHash)
	if stream == nil {
		return fmt.Errorf("failed to send ping")
	}

	reply, err := ngp2p.ReceiveReply(id, stream)
	if err != nil {
		return err
	}

	switch reply.Header.MessageType {
	case ngp2p.MessageType_PONG:
		pongPayload, err := ngp2p.DecodePongPayload(reply.Payload)
		if err != nil {
			return err
		}

		sync.PutRemote(peerID, &remoteRecord{
			id:     peerID,
			origin: pongPayload.Origin,
			latest: pongPayload.Latest,
		})

	case ngp2p.MessageType_REJECT:
		return fmt.Errorf("ping is rejected by remote: %s", string(reply.Payload))
	default:
		return fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}

	return nil
}
