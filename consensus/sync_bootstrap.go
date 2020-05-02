package consensus

import (
	"fmt"

	core "github.com/libp2p/go-libp2p-core"

	"github.com/ngchain/ngcore/ngp2p"
)

func (pow *PoWork) bootstrap() {
	for _, remotePeerID := range pow.localNode.Peerstore().Peers() {
		go pow.getRemoteStatus(remotePeerID)
	}
}

func (pow *PoWork) getRemoteStatus(peerID core.PeerID) error {
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
		pongPayload, err := ngp2p.GetPongPayload(reply.Payload)
		if err != nil {
			return err
		}
		pow.PutRemote(peerID, &remoteRecord{
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
