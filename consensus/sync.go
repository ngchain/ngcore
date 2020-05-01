package consensus

import (
	"fmt"

	core "github.com/libp2p/go-libp2p-core"

	"github.com/ngchain/ngcore/ngp2p"
)

func (c *PoWork) Init() {
	// init with a config, replacing main
}

func (c *PoWork) bootstrap() {
	for _, remotePeerID := range c.localNode.Peerstore().Peers() {
		go c.GetRemoteStatus(remotePeerID)
	}
}

func (c *PoWork) GetRemoteStatus(peerID core.PeerID) error {
	origin := c.chain.GetOriginBlock()
	latest := c.chain.GetLatestBlock()
	checkpointHash := c.chain.GetLatestCheckpointHash()

	id, s := c.localNode.Ping(peerID, origin.GetHeight(), latest.GetHeight(), checkpointHash)
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

	case ngp2p.MessageType_REJECT:
		return fmt.Errorf("ping is rejected by remote")
	default:
		return fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}

	return nil
}
