package ngp2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/utils"
)

// helper method - writes a protobuf go data object to a network stream.
func (n *LocalNode) sendProtoMessage(peerID peer.ID, method protocol.ID, data proto.Message) bool {
	raw, err := utils.Proto.Marshal(data)
	if err != nil {
		log.Error(err)

		return false
	}

	s, err := n.NewStream(context.Background(), peerID, method)
	if err != nil {
		log.Error(err)
		return false
	}

	if _, err = s.Write(raw); err != nil {
		log.Error(err)

		_ = s.Reset()

		return false
	}

	if err = helpers.FullClose(s); err != nil {
		log.Error(err)

		_ = s.Reset()

		return false
	}

	return true
}
