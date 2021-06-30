package wired

import (
	"context"
	"github.com/c0mm4nd/rlp"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-msgio"
	"google.golang.org/protobuf/proto"
)

// Send is a helper method - writes a protobuf go data object to a network stream.
// then the stream will be returned and caller is able to read the response from it.
func Send(host core.Host, protocolID protocol.ID, peerID peer.ID, data proto.Message) (network.Stream, error) {
	raw, err := rlp.EncodeToBytes(data)
	if err != nil {
		return nil, err
	}

	stream, err := host.NewStream(context.Background(), peerID, protocolID)
	if err != nil {
		return nil, err
	}

	w := msgio.NewWriter(stream)
	if err = w.WriteMsg(raw); err != nil {
		return nil, err
	}

	return stream, nil
}

func Reply(stream network.Stream, data proto.Message) error {
	raw, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	if err = msgio.NewWriter(stream).WriteMsg(raw); err != nil {
		return err
	}

	//// close the stream and waits to read an EOF from the other side.
	//err = stream.Close()
	//if err != nil {
	//	return err
	//}

	return nil
}
