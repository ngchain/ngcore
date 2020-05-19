package ngp2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/utils"
)

// sendProtoMessage is a helper method - writes a protobuf go data object to a network stream.
// then the stream will be returned and caller is able to read the response from it.
func (n *LocalNode) sendProtoMessage(peerID peer.ID, data proto.Message) (network.Stream, error) {
	raw, err := utils.Proto.Marshal(data)
	if err != nil {
		return nil, err
	}

	stream, err := n.NewStream(context.Background(), peerID, channal)
	if err != nil {
		return nil, err
	}

	if _, err = stream.Write(raw); err != nil {
		return nil, err
	}

	// Close stream for writing.
	// if err := stream.Close(); err != nil {
	// 	return nil, err
	// }

	return stream, nil
}

func (n *LocalNode) replyToStream(stream network.Stream, data proto.Message) error {
	raw, err := utils.Proto.Marshal(data)
	if err != nil {
		return err
	}

	if _, err = stream.Write(raw); err != nil {
		return err
	}

	// close the stream and waits to read an EOF from the other side.
	// err = helpers.FullClose(stream)
	// if err != nil {
	// 	return err
	// }

	return nil
}
