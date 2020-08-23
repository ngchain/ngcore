package message

import (
	"context"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-msgio"
	"github.com/ngchain/ngcore/ngp2p/defaults"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/utils"
)

// SendProtoMessage is a helper method - writes a protobuf go data object to a network stream.
// then the stream will be returned and caller is able to read the response from it.
func Send(host core.Host, peerID peer.ID, data proto.Message) (network.Stream, error) {
	raw, err := utils.Proto.Marshal(data)
	if err != nil {
		return nil, err
	}

	stream, err := host.NewStream(context.Background(), peerID, defaults.WiredProtocol)
	if err != nil {
		return nil, err
	}

	w := msgio.NewWriter(stream)
	if err = w.WriteMsg(raw); err != nil {
		return nil, err
	}

	// Close stream for writing.
	if err := stream.Close(); err != nil {
		return nil, err
	}

	return stream, nil
}

func Reply(stream network.Stream, data proto.Message) error {
	raw, err := utils.Proto.Marshal(data)
	if err != nil {
		return err
	}

	if err = msgio.NewWriter(stream).WriteMsg(raw); err != nil {
		return err
	}

	// close the stream and waits to read an EOF from the other side.
	err = helpers.FullClose(stream)
	if err != nil {
		return err
	}

	return nil
}
